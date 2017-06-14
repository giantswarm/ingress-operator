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
	// guest cluster. E.g.:
	//
	//     namespace/service:30010
	//     namespace/service:30011
	//
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

	createState, deleteState, err := s.analyseState(cState, dState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = s.createState(createState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = s.deleteState(deleteState)
	if err != nil {
		return microerror.MaskAny(err)
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
			namespace := customObject.HostCluster.IngressController.Namespace
			configMap := customObject.HostCluster.IngressController.ConfigMap

			k8sConfigMap, err = s.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap)
			if err != nil {
				return OperatorState{}, microerror.MaskAny(err)
			}
			cState.ConfigMap.ConfigMap = k8sConfigMap
		}

		for _, p := range customObject.ProtocolPorts {
			configMapValue := fmt.Sprintf(
				ConfigMapValueFormat,
				customObject.GuestCluster.Namespace,
				customObject.GuestCluster.Service,
				p.IngressPort,
			)

			for k, v := range k8sConfigMap {
				if configMapValue == v {
					cState.ConfigMap.Data[k] = v
					break
				}
			}
		}
	}

	// Lookup the current state of the service.
	{
		var k8sService *apiv1.Service
		{
			namespace := customObject.HostCluster.IngressController.Namespace
			service := customObject.HostCluster.IngressController.Service

			k8sService, err = s.k8sClient.CoreV1().Services(namespace).Get(service)
			if err != nil {
				return OperatorState{}, microerror.MaskAny(err)
			}
			cState.Service.Service = k8sService
		}

		for _, p := range customObject.ProtocolPorts {
			servicePortName := fmt.Sprintf(
				ServicePortNameFormat,
				p.Protocol,
				p.IngressPort,
				customObject.GuestCluster.ID,
			)

			for _, p := range k8sService.Spec.Ports {
				if servicePortName == p.Name {
					cState.Service.Ports = append(cState.Service.Ports, p)
					break
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
		for _, p := range customObject.ProtocolPorts {
			configMapKey := p.LBPort
			configMapValue := fmt.Sprintf(
				ConfigMapValueFormat,
				customObject.GuestCluster.Namespace,
				customObject.GuestCluster.Service,
				p.IngressPort,
			)

			dState.ConfigMap.Data[configMapKey] = configMapValue
		}
	}

	{
		for _, p := range customObject.ProtocolPorts {
			servicePortName := fmt.Sprintf(
				ServicePortNameFormat,
				p.Protocol,
				p.IngressPort,
				customObject.GuestCluster.ID,
			)

			newPort := apiv1.ServicePort{
				Name:       servicePortName,
				Protocol:   apiv1.ProtocolTCP,
				Port:       int32(p.LBPort),
				TargetPort: intstr.FromInt(p.LBPort),
				NodePort:   int32(p.LBPort),
			}

			dState.Service.Ports = append(dState.Service.Ports, newPort)
		}
	}

	return dState, nil
}

func (s *Service) analyseState(cState, dState OperatorState) (OperatorState, OperatorState, error) {
	var createState OperatorState
	var deleteState OperatorState

	// Make sure the current state of the Kubernetes resources is known by the
	// create and delete actions. The resources we already fetched represent the
	// source of truth and have to be used to update the resources in the next
	// steps.
	{
		createState.ConfigMap.ConfigMap = cState.ConfigMap.ConfigMap
		createState.Service.Service = cState.Service.Service

		deleteState.ConfigMap.ConfigMap = cState.ConfigMap.ConfigMap
		deleteState.Service.Service = cState.Service.Service
	}

	// Find anything which is in desired state but not in current state.
	// Everything we find here is supposed to be created.
	{
		// Process config-map to find its create state.
		{
			for k, v := range dState.ConfigMap.Data {
				if !inConfigMapData(cState.ConfigMap.Data, k, v) {
					createState.ConfigMap.Data[k] = v
				}
			}
		}

		// Process service to find its create state.
		{
			for _, p := range dState.Service.Ports {
				if !inServicePorts(cState.ServicePorts, p) {
					createState.Service.Ports = append(createState.Service.Ports, p)
				}
			}
		}
	}

	// Find anything which is in current state but not in desired state.
	// Everything we find here is supposed to be deleted.
	{
		// Process config-map to find its delete state.
		{
			for k, v := range cState.ConfigMap.Data {
				if !inConfigMapData(dState.ConfigMap.Data, k, v) {
					deleteState.ConfigMap.Data[k] = v
				}
			}
		}

		// Process service to find its delete state.
		{
			for _, p := range cState.Service.Ports {
				if !inServicePorts(dState.ServicePorts, p) {
					deleteState.Service.Ports = append(deleteState.Service.Ports, p)
				}
			}
		}
	}

	return createState, deleteState, nil
}

func (s *Service) createState(createState OperatorState) error {
	// TODO config-map
	// TODO service
	{
		k8sService.Spec.Ports = append(k8sService.Spec.Ports, newPort)

		_, err = s.k8sClient.CoreV1().Services(s.ingressControllerNamespace).Update(k8sService)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	return nil
}

func (s *Service) deleteState(deleteState OperatorState) error {
	return nil
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

func inConfigMapData(data map[string]string, k, v string) bool {
	for dk, dv := range data {
		if dk == k && dv == v {
			return true
		}
	}

	return false
}

func inServicePorts(ports []apiv1.ServicePort, p apiv1.ServicePort) bool {
	for _, pp := range ports {
		if pp.String() == p.String() {
			return true
		}
	}

	return false
}
