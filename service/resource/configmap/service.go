package configmap

import (
	"fmt"
	"strconv"

	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/client/k8s"
	"github.com/giantswarm/operatorkit/framework"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

const (
	// DataValueFormat is the format string used to create a config map data
	// value. It combines the namespace of the guest cluster, the service name
	// used to send traffic to and the port of the ingress controller within the
	// guest cluster. E.g.:
	//
	//     namespace/service:30010
	//     namespace/service:30011
	//
	DataValueFormat = "%s/%s:%d"
	// Name is the identifier of the resource.
	Name = "configmap"
)

// Config represents the configuration used to create a new config map resource.
type Config struct {
	// Dependencies.
	K8sClient kubernetes.Interface
	Logger    micrologger.Logger
}

// DefaultConfig provides a default configuration to create a new config map
// resource by best effort.
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

// New creates a new configured config map resource.
func New(config Config) (*Service, error) {
	// Dependencies.
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.K8sClient must not be empty")
	}
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Logger must not be empty")
	}

	newService := &Service{
		// Dependencies.
		k8sClient: config.K8sClient,
		logger:    config.Logger,
	}

	return newService, nil
}

// Service implements the config map resource.
type Service struct {
	// Dependencies.
	k8sClient kubernetes.Interface
	logger    micrologger.Logger
}

func (s *Service) GetCurrentState(obj interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get current state", "resource", "config-map")

	// Lookup the current state of the configmap.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	configMap := customObject.Spec.HostCluster.IngressController.ConfigMap
	k8sConfigMap, err := s.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap, apismetav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}
	// Ensure that the map is assignable. This prevents panics down the road in
	// case the config map has no data at all.
	if k8sConfigMap.Data == nil {
		k8sConfigMap.Data = map[string]string{}
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found k8s state: %#v", *k8sConfigMap), "resource", "config-map")

	return k8sConfigMap, nil
}

func (s *Service) GetDesiredState(obj interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get desired state", "resource", "config-map")

	// Lookup the desired state of the config map to have a reference of data how
	// it should be.
	dState := map[string]string{}
	for _, p := range customObject.Spec.ProtocolPorts {
		configMapKey := strconv.Itoa(p.LBPort)
		configMapValue := fmt.Sprintf(
			DataValueFormat,
			customObject.Spec.GuestCluster.Namespace,
			customObject.Spec.GuestCluster.Service,
			p.IngressPort,
		)

		dState[configMapKey] = configMapValue
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found desired state: %#v", dState), "resource", "config-map")

	return dState, nil
}

func (s *Service) GetCreateState(obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := currentState.(*apiv1.ConfigMap)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &apiv1.ConfigMap{}, currentState)
	}
	dState, ok := desiredState.(map[string]string)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", map[string]string{}, desiredState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get create state", "resource", "config-map")

	// Make sure the current state of the Kubernetes resources is known by the
	// create action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	createState := cState

	// Find anything which is in desired state but not in the current state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be created. Note that the creation
	// of config map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. creation of resources. In our
	// case here we only transform data within resources. Therefore the update.
	for k, v := range dState {
		if !inConfigMapData(createState.Data, k, v) {
			createState.Data[k] = v
		}
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found create state: %#v", createState), "resource", "config-map")

	return createState, nil
}

func (s *Service) GetDeleteState(obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := currentState.(*apiv1.ConfigMap)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &apiv1.ConfigMap{}, currentState)
	}
	dState, ok := desiredState.(map[string]string)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", map[string]string{}, desiredState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get delete state", "resource", "config-map")

	// Make sure the current state of the Kubernetes resources is known by the
	// delete action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	deleteState := cState

	// Find anything which is in current state but not in the desired state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be deleted. Note that the deletion
	// of config map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. deletion of resources. In our
	// case here we only transform data within resources. Therefore the update.
	newData := map[string]string{}
	for k, v := range deleteState.Data {
		if !inConfigMapData(dState, k, v) {
			newData[k] = v
		}
	}
	deleteState.Data = newData

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found delete state: %#v", deleteState), "resource", "config-map")

	return deleteState, nil
}

func (s *Service) Name() string {
	return Name
}

func (s *Service) ProcessCreateState(obj, createState interface{}) error {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := createState.(*apiv1.ConfigMap)
	if !ok {
		return microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &apiv1.ConfigMap{}, createState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "process create state", "resource", "config-map")

	// Add the config map key-value pairs by updating the Kubernetes config map
	// resource.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	_, err := s.k8sClient.CoreV1().ConfigMaps(namespace).Update(cState)
	if err != nil {
		return microerror.Mask(err)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "processed create state", "resource", "config-map")

	return nil
}

func (s *Service) ProcessDeleteState(obj, deleteState interface{}) error {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	dState, ok := deleteState.(*apiv1.ConfigMap)
	if !ok {
		return microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &apiv1.ConfigMap{}, deleteState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "process delete state", "resource", "config-map")

	// Add the config map key-value pairs by updating the Kubernetes config map
	// resource.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	_, err := s.k8sClient.CoreV1().ConfigMaps(namespace).Update(dState)
	if err != nil {
		return microerror.Mask(err)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "processed delete state", "resource", "config-map")

	return nil
}

func (s *Service) Underlying() framework.Resource {
	return s
}

func inConfigMapData(data map[string]string, k, v string) bool {
	for dk, dv := range data {
		if dk == k && dv == v {
			return true
		}
	}

	return false
}
