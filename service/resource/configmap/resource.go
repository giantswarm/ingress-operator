package configmap

import (
	"context"
	"fmt"
	"strconv"

	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
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
	return Config{
		// Dependencies.
		K8sClient: nil,
		Logger:    nil,
	}
}

// Resource implements the config map resource.
type Resource struct {
	// Dependencies.
	k8sClient kubernetes.Interface
	logger    micrologger.Logger
}

// New creates a new configured config map resource.
func New(config Config) (*Resource, error) {
	// Dependencies.
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.K8sClient must not be empty")
	}
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Logger must not be empty")
	}

	newResource := &Resource{
		// Dependencies.
		k8sClient: config.K8sClient,
		logger:    config.Logger,
	}

	return newResource, nil
}

func (r *Resource) GetCurrentState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get current state", "resource", "config-map")

	// Lookup the current state of the configmap.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	configMap := customObject.Spec.HostCluster.IngressController.ConfigMap
	k8sConfigMap, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap, apismetav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}
	// Ensure that the map is assignable. This prevents panics down the road in
	// case the config map has no data at all.
	if k8sConfigMap.Data == nil {
		k8sConfigMap.Data = map[string]string{}
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found k8s state: %#v", *k8sConfigMap), "resource", "config-map")

	return k8sConfigMap, nil
}

func (r *Resource) GetDesiredState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get desired state", "resource", "config-map")

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

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found desired state: %#v", dState), "resource", "config-map")

	return dState, nil
}

func (r *Resource) GetCreateState(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}
	currentConfigMap, err := toConfigMap(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.(map[string]string)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", map[string]string{}, desiredState)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get create state", "resource", "config-map")

	// Make sure the current state of the Kubernetes resources is known by the
	// create action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	createState := currentConfigMap

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

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found create state: %#v", createState), "resource", "config-map")

	return createState, nil
}

func (r *Resource) GetDeleteState(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}
	currentConfigMap, err := toConfigMap(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.(map[string]string)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", map[string]string{}, desiredState)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get delete state", "resource", "config-map")

	// Make sure the current state of the Kubernetes resources is known by the
	// delete action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	deleteState := currentConfigMap

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

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found delete state: %#v", deleteState), "resource", "config-map")

	return deleteState, nil
}

// GetUpdateState currently returns nil values because this is a simple resource
// not concerned with being updated, just fulfilling the resource interface
func (r *Resource) GetUpdateState(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, interface{}, interface{}, error) {
	return nil, nil, nil, nil
}

func (r *Resource) Name() string {
	return Name
}

func (r *Resource) ProcessCreateState(ctx context.Context, obj, createState interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	configMapToCreate, err := toConfigMap(createState)
	if err != nil {
		return microerror.Mask(err)
	}

	if configMapToCreate != nil {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "creating the config map data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Update(configMapToCreate)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "created the config map data in the Kubernetes API")
	} else {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "the config map data does not need to be created from the Kubernetes API")
	}

	return nil
}

func (r *Resource) ProcessDeleteState(ctx context.Context, obj, deleteState interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	configMapToDelete, err := toConfigMap(deleteState)
	if err != nil {
		return microerror.Mask(err)
	}

	if configMapToDelete != nil {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "deleting the config map data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Update(configMapToDelete)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "deleted the config map data in the Kubernetes API")
	} else {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "the config map data does not need to be deleted in the Kubernetes API")
	}

	return nil
}

// ProcessUpdateState currently returns a nil value because this is a simple
// resource not concerned with being updated, just fulfilling the resource
// interface
func (r *Resource) ProcessUpdateState(ctx context.Context, obj, updateState interface{}) error {
	return nil
}

func (r *Resource) Underlying() framework.Resource {
	return r
}

func inConfigMapData(data map[string]string, k, v string) bool {
	for dk, dv := range data {
		if dk == k && dv == v {
			return true
		}
	}

	return false
}

func toCustomObject(v interface{}) (ingresstpr.CustomObject, error) {
	customObjectPointer, ok := v.(*ingresstpr.CustomObject)
	if !ok {
		return ingresstpr.CustomObject{}, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, v)
	}
	customObject := *customObjectPointer

	return customObject, nil
}

func toConfigMap(v interface{}) (*apiv1.ConfigMap, error) {
	if v == nil {
		return nil, nil
	}

	configMaps, ok := v.(*apiv1.ConfigMap)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &apiv1.ConfigMap{}, v)
	}

	return configMaps, nil
}