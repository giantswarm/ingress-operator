package operator

import (
	"fmt"
	"strconv"
	"sync"

	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/runtime"
	"k8s.io/client-go/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"

	"github.com/giantswarm/ingresstpr"
	microerror "github.com/giantswarm/microkit/error"
	micrologger "github.com/giantswarm/microkit/logger"
	"github.com/giantswarm/operatorkit/client/k8s"
	"github.com/giantswarm/operatorkit/tpr"
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

	var err error
	var newTPR *tpr.TPR
	{
		tprConfig := tpr.DefaultConfig()

		tprConfig.K8sClient = config.K8sClient
		tprConfig.Logger = config.Logger

		tprConfig.Description = ingresstpr.Description
		tprConfig.Name = ingresstpr.Name
		tprConfig.Version = ingresstpr.VersionV1

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
			NewObjectFunc:     func() runtime.Object { return &ingresstpr.CustomObject{} },
			NewObjectListFunc: func() runtime.Object { return &ingresstpr.List{} },
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
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	cState, err := s.getCurrentState(*customObject)
	if err != nil {
		return microerror.MaskAny(err)
	}

	dState, err := s.getDesiredState(*customObject)
	if err != nil {
		return microerror.MaskAny(err)
	}

	createState, err := s.getCreateState(*customObject, cState, dState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = s.processCreateState(*customObject, createState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	deleteState, err := s.getDeleteState(*customObject, cState, dState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = s.processDeleteState(*customObject, deleteState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	return nil
}

func (s *Service) getCurrentState(customObject ingresstpr.CustomObject) (OperatorState, error) {
	var err error
	var cState OperatorState

	// Lookup the current state of the configmap.
	{
		var k8sConfigMap *apiv1.ConfigMap
		{
			namespace := customObject.Spec.HostCluster.IngressController.Namespace
			configMap := customObject.Spec.HostCluster.IngressController.ConfigMap

			k8sConfigMap, err = s.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap)
			if err != nil {
				return OperatorState{}, microerror.MaskAny(err)
			}
			cState.ConfigMap.Resource = *k8sConfigMap
		}

		for _, p := range customObject.Spec.ProtocolPorts {
			configMapValue := fmt.Sprintf(
				ConfigMapValueFormat,
				customObject.Spec.GuestCluster.Namespace,
				customObject.Spec.GuestCluster.Service,
				p.IngressPort,
			)

			for k, v := range k8sConfigMap.Data {
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
			namespace := customObject.Spec.HostCluster.IngressController.Namespace
			service := customObject.Spec.HostCluster.IngressController.Service

			k8sService, err = s.k8sClient.CoreV1().Services(namespace).Get(service)
			if err != nil {
				return OperatorState{}, microerror.MaskAny(err)
			}
			cState.Service.Resource = *k8sService
		}

		for _, p := range customObject.Spec.ProtocolPorts {
			servicePortName := fmt.Sprintf(
				ServicePortNameFormat,
				p.Protocol,
				p.IngressPort,
				customObject.Spec.GuestCluster.ID,
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

func (s *Service) getDesiredState(customObject ingresstpr.CustomObject) (OperatorState, error) {
	var dState OperatorState

	{
		for _, p := range customObject.Spec.ProtocolPorts {
			configMapKey := strconv.Itoa(p.LBPort)
			configMapValue := fmt.Sprintf(
				ConfigMapValueFormat,
				customObject.Spec.GuestCluster.Namespace,
				customObject.Spec.GuestCluster.Service,
				p.IngressPort,
			)

			dState.ConfigMap.Data[configMapKey] = configMapValue
		}
	}

	{
		for _, p := range customObject.Spec.ProtocolPorts {
			servicePortName := fmt.Sprintf(
				ServicePortNameFormat,
				p.Protocol,
				p.IngressPort,
				customObject.Spec.GuestCluster.ID,
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

func (s *Service) getCreateState(customObject ingresstpr.CustomObject, cState, dState OperatorState) (ActionState, error) {
	var createState ActionState

	// Make sure the current state of the Kubernetes resources is known by the
	// create and delete actions. The resources we already fetched represent the
	// source of truth and have to be used to update the resources in the next
	// steps.
	{
		createState.ConfigMap = cState.ConfigMap.Resource
		createState.Service = cState.Service.Resource
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
				if !inServicePorts(cState.Service.Ports, p) {
					createState.Service.Spec.Ports = append(createState.Service.Spec.Ports, p)
				}
			}
		}
	}

	return createState, nil
}

func (s *Service) getDeleteState(customObject ingresstpr.CustomObject, cState, dState OperatorState) (ActionState, error) {
	var deleteState ActionState

	// Make sure the current state of the Kubernetes resources is known by the
	// create and delete actions. The resources we already fetched represent the
	// source of truth and have to be used to update the resources in the next
	// steps.
	{
		deleteState.ConfigMap = cState.ConfigMap.Resource
		deleteState.Service = cState.Service.Resource
	}

	// Find anything which is in current state but not in desired state.
	// Everything we find here is supposed to be deleted.
	{
		// Process config-map to find its delete state.
		{
			newData := map[string]string{}
			for k, v := range cState.ConfigMap.Data {
				if !inConfigMapData(dState.ConfigMap.Data, k, v) {
					newData[k] = v
				}
			}
			deleteState.ConfigMap.Data = newData
		}

		// Process service to find its delete state.
		{
			var newPorts []apiv1.ServicePort
			for _, p := range cState.Service.Ports {
				if !inServicePorts(dState.Service.Ports, p) {
					newPorts = append(newPorts, p)
				}
			}
			deleteState.Service.Spec.Ports = newPorts
		}
	}

	return deleteState, nil
}

func (s *Service) processCreateState(customObject ingresstpr.CustomObject, createState ActionState) error {
	// Add the config-map key-value pairs by updating the Kubernetes config-map
	// resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().ConfigMaps(namespace).Update(&createState.ConfigMap)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	// Add the service ports by updating the Kubernetes service resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().Services(namespace).Update(&createState.Service)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	return nil
}

func (s *Service) processDeleteState(customObject ingresstpr.CustomObject, deleteState ActionState) error {
	// Add the config-map key-value pairs by updating the Kubernetes config-map
	// resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().ConfigMaps(namespace).Update(&deleteState.ConfigMap)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	// Add the service ports by updating the Kubernetes service resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().Services(namespace).Update(&deleteState.Service)
		if err != nil {
			return microerror.MaskAny(err)
		}
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
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	cState, err := s.getCurrentState(*customObject)
	if err != nil {
		return microerror.MaskAny(err)
	}

	deleteState, err := s.getDeleteState(*customObject, cState, OperatorState{})
	if err != nil {
		return microerror.MaskAny(err)
	}

	err = s.processDeleteState(*customObject, deleteState)
	if err != nil {
		return microerror.MaskAny(err)
	}

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
