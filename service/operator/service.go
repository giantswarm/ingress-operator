package operator

import (
	"flag"
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"

	"github.com/giantswarm/kvmtpr"
	microerror "github.com/giantswarm/microkit/error"
	micrologger "github.com/giantswarm/microkit/logger"
	"github.com/giantswarm/operatorkit/client/k8s"
	"github.com/giantswarm/operatorkit/tpr"
	"github.com/spf13/viper"
)

const (
	// ConfigMapValueFormat is the format string used to create a config-map
	// value. It combines the namespace of the guest cluster, the service name
	// used to send traffic to and the port of the ingress controller within the
	// guest cluster.
	ConfigMapValueFormat = "%s/%s:%d"
	// ServicePortNameFormat is the format string used to create a service port
	// name. It combines the protocol, the port of the ingress controller within
	// the guest cluster and the guest cluster ID, in this order. E.g.:
	//
	//     http-30010-al9qy
	//     https-30011-al9qy
	//
	ServicePortNameFormat = "%s-%s-%s"
)

// Config represents the configuration used to create a new service.
type Config struct {
	// Dependencies.
	K8sClient kubernetes.Interface
	Logger    micrologger.Logger

	// Settings.
	Flag  *flag.Flag
	Viper *viper.Viper
}

// DefaultConfig provides a default configuration to create a new service by
// best effort.
func DefaultConfig() Config {
	var err error

	var k8sClient kubernetes.Interface
	{
		config := k8s.DefaultConfig()
		k8sClient, err = k8s.NewClient(config)
		if err != nil {
			panic(err)
		}
	}

	var newLogger micrologger.Logger
	{
		config := micrologger.DefaultConfig()
		newLogger, err = micrologger.New(config)
		if err != nil {
			panic(err)
		}
	}

	return Config{
		// Dependencies.
		K8sClient: k8sClient,
		Logger:    newLogger,

		// Settings.
		Flag:  nil,
		Viper: nil,
	}
}

// New creates a new configured service.
func New(config Config) (*Service, error) {
	// Dependencies.
	if config.K8sClient == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.K8sClient must not be empty")
	}
	if config.Logger == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.Logger must not be empty")
	}

	// Settings.
	if config.Flag == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.Flag must not be empty")
	}
	if config.Viper == nil {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.Viper must not be empty")
	}

	var err error
	var newTPR *tpr.TPR
	{
		tprConfig := tpr.DefaultConfig()

		tprConfig.K8sClient = config.K8sClient
		tprConfig.Logger = config.Logger

		tprConfig.Description = kvmtpr.Description
		tprConfig.Name = kvmtpr.Name
		tprConfig.Version = kvmtpr.VersionV1

		newTPR, err = tpr.New(tprConfig)
		if err != nil {
			return nil, microerror.MaskAny(err)
		}
	}

	newService := &Service{
		// Dependencies.
		k8sClient: config.K8sClient,
		logger:    config.Logger,

		// Internals
		bootOnce: sync.Once{},
		mutex:    sync.Mutex{},
		tpr:      newTPR,

		// Settings.
		flag:  config.Flag,
		viper: config.Viper,
	}

	return newService, nil
}

// Service implements the service.
type Service struct {
	// Dependencies.
	k8sClient kubernetes.Interface
	logger    micrologger.Logger

	// Internals.
	bootOnce sync.Once
	mutex    sync.Mutex
	tpr      *tpr.TPR

	// Settings.
	flag  *flag.Flag
	viper *viper.Viper
}

// Boot starts the service and implements the watch for the cluster TPR.
func (s *Service) Boot() {
	s.bootOnce.Do(func() {
		err := s.tpr.CreateAndWait()
		if tpr.IsAlreadyExists(err) {
			s.logger.Log("debug", "third party resource already exists")
		} else if err != nil {
			s.logger.Log("error", fmt.Sprintf("%#v", err))
			return
		}

		s.logger.Log("debug", "starting list/watch")

		newResourceEventHandler := &cache.ResourceEventHandlerFuncs{
			AddFunc:    s.addFunc,
			DeleteFunc: s.deleteFunc,
		}
		newZeroObjectFactory := &tpr.ZeroObjectFactoryFuncs{
			NewObjectFunc:     func() runtime.Object { return &kvmtpr.CustomObject{} },
			NewObjectListFunc: func() runtime.Object { return &kvmtpr.List{} },
		}

		s.tpr.NewInformer(newResourceEventHandler, newZeroObjectFactory).Run(nil)
	})
}

func (s *Service) addFunc(obj interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	err := s.addFuncError(obj)
	if err != nil {
		s.logger.Log("error", fmt.Sprintf("%#v", err), "event", "create")
	}
}

func (s *Service) addFuncError(obj interface{}) error {
	customObject, ok := obj.(*kvmtpr.CustomObject)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &kvmtpr.CustomObject{}, obj)
	}

	cState, err := s.currentState(*customObject)
	if err != nil {
		return microerror.MaskAny(err)
	}

	dState, err := s.desiredState(*customObject)
	if err != nil {
		return microerror.MaskAny(err)
	}

	aState, err := s.analyseState(cState, dState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	for _, s := range aState.CreateState {
		err := s.createState(s)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	for _, s := range aState.DeleteState {
		err = s.deleteState(s)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	for _, s := range aState.UpdateState {
		err = s.updateState(s)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	return nil
}

func (s *Service) currentState(customObject kvmtpr.CustomObject) (OperatorState, error) {
	var err error
	var cState OperatorState

	// Lookup the current state of the configmap.
	{
		var k8sConfigMap *apiv1.ConfigMap
		{
			namespace := s.viper.GetString(s.flag.Service.HostCluster.IngressController.Namespace)
			configMap := s.viper.GetString(s.flag.Service.HostCluster.IngressController.ConfigMap)

			k8sConfigMap, err = s.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap)
			if err != nil {
				return OperatorState{}, microerror.MaskAny(err)
			}
		}

		{
			protocolPorts := s.viper.GetStringMap(s.flag.Service.GuestCluster.IngressController.ProtocolPorts)
			namespace := ClusterNamespace(customObject)
			service := s.viper.GetStringMap(s.flag.Service.GuestCluster.Service)

			for _, port := range protocolPorts {
				configMapValue := fmt.Sprintf(ConfigMapValueFormat, namespace, service, port)

				for k, v := range k8sConfigMap {
					if configMapValue == v {
						cState.ConfigMap.Values[k] = v
						break
					}
				}
			}
		}
	}

	// Lookup the current state of the service.
	{
		var k8sService *apiv1.Service
		{
			namespace := s.viper.GetString(s.flag.Service.HostCluster.IngressController.Namespace)
			service := s.viper.GetString(s.flag.Service.HostCluster.IngressController.Service)

			k8sService, err = s.k8sClient.CoreV1().Services(namespace).Get(service)
			if err != nil {
				return OperatorState{}, microerror.MaskAny(err)
			}
		}

		{
			protocolPorts := s.viper.GetStringMap(s.flag.Service.GuestCluster.IngressController.ProtocolPorts)
			clusterID := ClusterID(customObject)

			for protocol, port := range protocolPorts {
				servicePortName := fmt.Sprintf(ServicePortNameFormat, protocol, port, clusterID)

				for _, p := range k8sService.Spec.Ports {
					if servicePortName == p.Name {
						cState.Service.Ports = append(cState.Service.Ports, p)
						break
					}
				}
			}
		}
	}

	return cState, nil
}

func (s *Service) desiredState(customObject kvmtpr.CustomObject) (OperatorState, error) {
	var err error
	var dState OperatorState

	{
		protocolPorts := s.viper.GetStringMap(s.flag.Service.GuestCluster.IngressController.ProtocolPorts)
		namespace := ClusterNamespace(customObject)
		service := s.viper.GetStringMap(s.flag.Service.GuestCluster.Service)

		for _, port := range protocolPorts {
			configMapKey := "k" // TODO
			configMapValue := fmt.Sprintf(ConfigMapValueFormat, namespace, service, port)

			dState.ConfigMap.Values[configMapKey] = configMapValue
		}
	}

	{
		protocolPorts := s.viper.GetStringMap(s.flag.Service.GuestCluster.IngressController.ProtocolPorts)
		clusterID := ClusterID(customObject)

		for protocol, port := range protocolPorts {
			lbPort := 0 // TODO
			newPort := apiv1.ServicePort{
				Name:       fmt.Sprintf(ServicePortNameFormat, protocol, port, clusterID),
				Protocol:   apiv1.ProtocolTCP,
				Port:       int32(lbPort),
				TargetPort: intstr.FromInt(lbPort),
				NodePort:   int32(lbPort),
			}

			dState.Service.Ports = append(dState.Service.Ports, p)
		}
	}

	return dState, nil
}

//
//
//
//
//
//
// TODO
//
//
//
//
//

//	configmapPortResults, err := s.getPortForConfigMap(*customObject)
//	if err != nil {
//		return microerror.MaskAny(err)
//	}
//
//	servicePortResults := s.getPortForService(*customObject)
//	if err != nil {
//		return microerror.MaskAny(err)
//	}
//
//	err := validateportResults(configmapPortResults, servicePortResults)
//	if err != nil {
//		// TODO in such a case we could probably implement the ingress-operator to
//		// self heal the situation by just fixing the service/configmap. For now we
//		// do not care though.
//		return microerror.MaskAny(err)
//	}
//
//	for _, result := range configmapPortResults {
//		if result.Exists {
//			s.logger.Log("debug", fmt.Sprintf("port '%d' for cluster '%s' already exists in configmap", result.PortNumber, ClusterID(customObject)))
//		} else {
//			err := s.addPortToConfigMap(result.LBPort, result.Destination, k8sConfigMap)
//			if err != nil {
//				return microerror.MaskAny(err)
//			}
//		}
//	}
//
//	for _, result := range servicePortResults {
//		if result.Exists {
//			s.logger.Log("debug", fmt.Sprintf("port '%d' for cluster '%s' already exists in service", result.PortNumber, ClusterID(customObject)))
//		} else {
//			err := s.addPortToService(result.LBPort, k8sService)
//			if err != nil {
//				return microerror.MaskAny(err)
//			}
//		}
//	}

type configmapPortResult struct {
	Destination  string
	Exists       bool
	K8sConfigMap apiv1.ConfigMap
	LBPort       string
}

// getPortForConfigMap looks up a cluster port and its corresponding
// namespace/service mapping within the Kubernetes configmap resource of the
// configured ingress controller. Additionally a boolean is returned indicating
// if the port already exists within the configmap resource.
func (s *Service) getPortForConfigMap(customObject kvmtpr.CustomObject) ([]configmapPortResult, error) {
	k8sConfigMap, err := s.k8sClient.CoreV1().ConfigMaps(s.ingressControllerNamespace).Get(s.configmap)
	if err != nil {
		return nil, microerror.MaskAny(err)
	}

	portName := PortName(customObject)
	port, exists := serviceToPortByName(k8sService, portName)
	if exists {
		result := configmapPortResult{
			Exists:     true,
			K8sService: k8sService,
			PortName:   portName,
			PortNumber: existingPortNumber,
		}
		return result, nil
	}

	lbPort, err := s.newLBPort(configmapToPorts(k8sConfigMap), s.availablePorts)
	if err != nil {
		return 0, false, apiv1.ConfigMap{}, microerror.MaskAny(err)
	}

	return port, false, k8sConfigMap, nil
}

func (s *Service) addPortToConfigMap(lbPort int, k8sConfigMap apiv1.ConfigMap) error {
	// TODO
	newPort := apiv1.ServicePort{
		Name:       portName,
		Protocol:   apiv1.ProtocolTCP,
		Port:       int32(lbPort),
		TargetPort: intstr.FromInt(lbPort),
		NodePort:   int32(lbPort),
	}

	k8sService.Spec.Ports = append(k8sService.Spec.Ports, newPort)

	_, err = s.k8sClient.CoreV1().Services(s.ingressControllerNamespace).Update(k8sService)
	if err != nil {
		return microerror.MaskAny(err)
	}

	return nil
}

type servicePortResult struct {
	Exists     bool
	K8sService apiv1.Service
	LBPort     int
	PortName   string
}

// getPortForService looks up a cluster port within the Kubernetes service
// resource of the configured ingress controller. Additionally a boolean is
// returned indicating if the port already exists within the service resource.
func (s *Service) getPortForService(customObject kvmtpr.CustomObject) ([]servicePortResult, error) {
	k8sService, err := s.k8sClient.CoreV1().Services(s.ingressControllerNamespace).Get(s.service)
	if err != nil {
		return nil, microerror.MaskAny(err)
	}

	var results []servicePortResult

	for k, v := range s.ingressControllerPorts {
		portName := fmt.Sprintf(PortNameFormat, k, v, ClusterID(customObject))
		existingPortNumber, exists := serviceToPortByName(k8sService, portName)
		if exists {
			result := servicePortResult{
				Exists:     true,
				K8sService: k8sService,
				LBPort:     existingPortNumber,
				PortName:   portName,
			}
			results = append(results, result)
		}
	}

	// As soon as we collected all the port information for the service, we can
	// stop our work here and return to continue other routines.
	if len(results) == len(s.ingressControllerPorts) {
		return results, nil
	}

	// Diff results and only apply new ones.

	for k, v := range s.ingressControllerPorts {
		newPortNumber, err := s.newLBPort(serviceToPorts(k8sService), s.availablePorts)
		if err != nil {
			return servicePortResult{}, microerror.MaskAny(err)
		}
		result := servicePortResult{
			Exists:     true,
			K8sService: k8sService,
			LBPort:     newPortNumber,
			PortName:   portName,
		}
		overwritingResults = append(overwritingResults, result)
	}

	return overwritingResults, nil
}

func (s *Service) addPortToService(lbPort int, k8sService apiv1.Service) error {
	newPort := apiv1.ServicePort{
		Name:       portName,
		Protocol:   apiv1.ProtocolTCP,
		Port:       int32(lbPort),
		TargetPort: intstr.FromInt(lbPort),
		NodePort:   int32(lbPort),
	}

	k8sService.Spec.Ports = append(k8sService.Spec.Ports, newPort)

	_, err = s.k8sClient.CoreV1().Services(s.ingressControllerNamespace).Update(k8sService)
	if err != nil {
		return microerror.MaskAny(err)
	}

	return nil
}

func (s *Service) deleteFunc(obj interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	err := s.deleteFuncError(obj)
	if err != nil {
		s.logger.Log("error", fmt.Sprintf("%#v", err), "event", "delete")
	}
}

func (s *Service) deleteFuncError(obj interface{}) error {
	return nil
}

func (s *Service) newLBPort(usedPorts []int, availablePorts []int) (int, error) {
	for _, a := range availablePorts {
		if portNumberExists(usedPorts, a) {
			continue
		}

		return a, nil
	}

	return 0, microerror.MaskAnyf(capacityReachedError, "no more ports available")
}

func portNumberExists(ports []int, number int) bool {
	for _, p := range ports {
		if p == number {
			return true
		}
	}

	return false
}

func validateportResults(servicePortResults []servicePortResult, configmapPortResults []configmapPortResults) error {
	// TODO
	return nil
}
