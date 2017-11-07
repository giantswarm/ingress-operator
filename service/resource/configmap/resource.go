package configmap

import (
	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/framework"
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

func (r *Resource) Name() string {
	return Name
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
