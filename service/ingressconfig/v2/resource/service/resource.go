package servicev2

import (
	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/framework"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// Name is the identifier of the resource.
	Name = "servicev2"
	// PortNameFormat is the format string used to create a service port name. It
	// combines the protocol, the port of the ingress controller within the guest
	// cluster and the guest cluster ID, in this order. E.g.:
	//
	//     http-30010-al9qy
	//     https-30011-al9qy
	//
	PortNameFormat = "%s-%d-%s"
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
	return Config{
		// Dependencies.
		K8sClient: nil,
		Logger:    nil,
	}
}

// Resource implements the service.
type Resource struct {
	// Dependencies.
	k8sClient kubernetes.Interface
	logger    micrologger.Logger
}

// New creates a new configured service.
func New(config Config) (*Resource, error) {
	// Dependencies.
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.K8sClient must not be empty")
	}
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Logger must not be empty")
	}

	newService := &Resource{
		// Dependencies.
		k8sClient: config.K8sClient,
		logger:    config.Logger.With("resource", Name),
	}

	return newService, nil
}

func (r *Resource) Name() string {
	return Name
}

func (r *Resource) Underlying() framework.Resource {
	return r
}

func inServicePorts(ports []apiv1.ServicePort, p apiv1.ServicePort) bool {
	for _, pp := range ports {
		if pp.String() == p.String() {
			return true
		}
	}

	return false
}

func getServicePortByPort(list []apiv1.ServicePort, item int32) (apiv1.ServicePort, error) {
	for _, p := range list {
		if p.Port == item {
			return p, nil
		}
	}

	return apiv1.ServicePort{}, microerror.Maskf(servicePortNotFoundError, "no service port with port '%d'", item)
}

func toCustomObject(v interface{}) (v1alpha1.IngressConfig, error) {
	customObjectPointer, ok := v.(*v1alpha1.IngressConfig)
	if !ok {
		return v1alpha1.IngressConfig{}, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &v1alpha1.IngressConfig{}, v)
	}
	customObject := *customObjectPointer

	return customObject, nil
}

func toService(v interface{}) (*apiv1.Service, error) {
	if v == nil {
		return nil, nil
	}

	services, ok := v.(*apiv1.Service)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &apiv1.Service{}, v)
	}

	return services, nil
}
