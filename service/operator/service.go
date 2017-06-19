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
	"github.com/giantswarm/operatorkit/operator"
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
	ServicePortNameFormat = "%s-%d-%s"
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
	// We lock the addFunc/deleteFunc to make sure only one addFunc/deleteFunc is
	// executed at a time. addFunc/deleteFunc is not thread safe. This is
	// important because the source of truth for the ingress-operator are
	// Kubernetes resources. In case we would run the operator logic in parallel,
	// we would run into race conditions.
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.logger.Log("debug", "executing the operator's addFunc")

	err := operator.ProcessCreate(obj, s)
	if err != nil {
		s.logger.Log("error", fmt.Sprintf("%#v", err), "event", "create")
	}
}

func (s *Service) deleteFunc(obj interface{}) {
	// We lock the addFunc/deleteFunc to make sure only one addFunc/deleteFunc is
	// executed at a time. addFunc/deleteFunc is not thread safe. This is
	// important because the source of truth for the ingress-operator are
	// Kubernetes resources. In case we would run the operator logic in parallel,
	// we would run into race conditions.
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.logger.Log("debug", "executing the operator's deleteFunc")

	err := operator.ProcessDelete(obj, s)
	if err != nil {
		s.logger.Log("error", fmt.Sprintf("%#v", err), "event", "delete")
	}
}

func (s *Service) GetCurrentState(obj interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	s.logger.Log("debug", "get current state", "cluster", customObject.Spec.GuestCluster.ID)

	var err error
	var cState CurrentState

	// Lookup the current state of the configmap.
	{
		var k8sConfigMap *apiv1.ConfigMap
		{
			namespace := customObject.Spec.HostCluster.IngressController.Namespace
			configMap := customObject.Spec.HostCluster.IngressController.ConfigMap

			k8sConfigMap, err = s.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap)
			if err != nil {
				return nil, microerror.MaskAny(err)
			}
			// Ensure that the map is assignable. This prevents panics down the road
			// in case the config-map has no data at all.
			if len(k8sConfigMap.Data) == 0 {
				k8sConfigMap.Data = map[string]string{}
			}
			cState.ConfigMap = *k8sConfigMap
			s.logger.Log("debug", fmt.Sprintf("found k8s config-map: %#v", *k8sConfigMap), "cluster", customObject.Spec.GuestCluster.ID)
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
				return nil, microerror.MaskAny(err)
			}
			cState.Service = *k8sService
			s.logger.Log("debug", fmt.Sprintf("found k8s service: %#v", *k8sService), "cluster", customObject.Spec.GuestCluster.ID)
		}
	}

	return cState, nil
}

func (s *Service) GetDesiredState(obj interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	s.logger.Log("debug", "get desired state", "cluster", customObject.Spec.GuestCluster.ID)

	var dState DesiredState

	// Lookup the desired state of the config-map to have a reference of data how
	// it should be.
	{
		dState.ConfigMapData = map[string]string{}
		for _, p := range customObject.Spec.ProtocolPorts {
			configMapKey := strconv.Itoa(p.LBPort)
			configMapValue := fmt.Sprintf(
				ConfigMapValueFormat,
				customObject.Spec.GuestCluster.Namespace,
				customObject.Spec.GuestCluster.Service,
				p.IngressPort,
			)

			dState.ConfigMapData[configMapKey] = configMapValue
		}
		s.logger.Log("debug", fmt.Sprintf("calculated desired state for k8s config-map: %#v", dState.ConfigMapData), "cluster", customObject.Spec.GuestCluster.ID)
	}

	// Lookup the desired state of the service to have a reference of ports how
	// they should be.
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

			dState.ServicePorts = append(dState.ServicePorts, newPort)
		}
		s.logger.Log("debug", fmt.Sprintf("calculated desired state for k8s service: %#v", dState.ServicePorts), "cluster", customObject.Spec.GuestCluster.ID)
	}

	return dState, nil
}

func (s *Service) GetEmptyState() interface{} {
	return DesiredState{}
}

func (s *Service) GetCreateState(obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := currentState.(CurrentState)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", CurrentState{}, currentState)
	}
	dState, ok := desiredState.(DesiredState)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", DesiredState{}, desiredState)
	}

	s.logger.Log("debug", "get create state", "cluster", customObject.Spec.GuestCluster.ID)

	var createState ActionState

	// Make sure the current state of the Kubernetes resources is known by the
	// create action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	{
		createState.ConfigMap = cState.ConfigMap
		createState.Service = cState.Service
	}

	// Find anything which is in desired state but not in the current state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be created. Note that the creation
	// of config-map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. creation of resources. In our
	// case here we only transform data within resources. Therefore the update.
	{
		// Process config-map to find its create state.
		{
			for k, v := range dState.ConfigMapData {
				if !inConfigMapData(createState.ConfigMap.Data, k, v) {
					createState.ConfigMap.Data[k] = v
				}
			}
			s.logger.Log("debug", fmt.Sprintf("calculated create state for k8s config-map: %#v", createState.ConfigMap), "cluster", customObject.Spec.GuestCluster.ID)
		}

		// Process service to find its create state.
		{
			for _, p := range dState.ServicePorts {
				if !inServicePorts(createState.Service.Spec.Ports, p) {
					createState.Service.Spec.Ports = append(createState.Service.Spec.Ports, p)
				}
			}
			s.logger.Log("debug", fmt.Sprintf("calculated create state for k8s service: %#v", createState.Service), "cluster", customObject.Spec.GuestCluster.ID)
		}
	}

	return createState, nil
}

func (s *Service) GetDeleteState(obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := currentState.(CurrentState)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", CurrentState{}, currentState)
	}
	dState, ok := desiredState.(DesiredState)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", DesiredState{}, desiredState)
	}

	s.logger.Log("debug", "get delete state", "cluster", customObject.Spec.GuestCluster.ID)

	var deleteState ActionState

	// Make sure the current state of the Kubernetes resources is known by the
	// delete action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	{
		deleteState.ConfigMap = cState.ConfigMap
		deleteState.Service = cState.Service
	}

	// Find anything which is in current state but not in the desired state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be deleted. Note that the deletion
	// of config-map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. deletion of resources. In our
	// case here we only transform data within resources. Therefore the update.
	{
		// Process config-map to find its delete state.
		{
			newData := map[string]string{}
			for k, v := range deleteState.ConfigMap.Data {
				if !inConfigMapData(dState.ConfigMapData, k, v) {
					newData[k] = v
				}
			}
			deleteState.ConfigMap.Data = newData
			s.logger.Log("debug", fmt.Sprintf("calculated delete state for k8s config-map: %#v", deleteState.ConfigMap), "cluster", customObject.Spec.GuestCluster.ID)
		}

		// Process service to find its delete state.
		{
			var newPorts []apiv1.ServicePort
			for _, p := range deleteState.Service.Spec.Ports {
				if !inServicePorts(dState.ServicePorts, p) {
					newPorts = append(newPorts, p)
				}
			}
			deleteState.Service.Spec.Ports = newPorts
		}
		s.logger.Log("debug", fmt.Sprintf("calculated delete state for k8s service: %#v", deleteState.Service), "cluster", customObject.Spec.GuestCluster.ID)
	}

	return deleteState, nil
}

func (s *Service) ProcessCreateState(obj, createState interface{}) error {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := createState.(ActionState)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", ActionState{}, createState)
	}

	s.logger.Log("debug", "process create state", "cluster", customObject.Spec.GuestCluster.ID)

	// Add the config-map key-value pairs by updating the Kubernetes config-map
	// resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().ConfigMaps(namespace).Update(&cState.ConfigMap)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	// Add the service ports by updating the Kubernetes service resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().Services(namespace).Update(&cState.Service)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	return nil
}

func (s *Service) ProcessDeleteState(obj, deleteState interface{}) error {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	dState, ok := deleteState.(ActionState)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", ActionState{}, deleteState)
	}

	s.logger.Log("debug", "process delete state", "cluster", customObject.Spec.GuestCluster.ID)

	// Add the config-map key-value pairs by updating the Kubernetes config-map
	// resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().ConfigMaps(namespace).Update(&dState.ConfigMap)
		if err != nil {
			return microerror.MaskAny(err)
		}
	}

	// Add the service ports by updating the Kubernetes service resource.
	{
		namespace := customObject.Spec.HostCluster.IngressController.Namespace

		_, err := s.k8sClient.CoreV1().Services(namespace).Update(&dState.Service)
		if err != nil {
			return microerror.MaskAny(err)
		}
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
