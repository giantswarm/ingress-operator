package service

import (
	"context"
	"fmt"

	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/framework"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

const (
	// Name is the identifier of the resource.
	Name = "service"
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
		logger:    config.Logger,
	}

	return newService, nil
}

func (r *Resource) GetCurrentState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get current state", "resource", "service")

	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	service := customObject.Spec.HostCluster.IngressController.Service
	k8sService, err := r.k8sClient.CoreV1().Services(namespace).Get(service, apismetav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found k8s state: %#v", *k8sService), "resource", "service")

	return k8sService, nil
}

func (r *Resource) GetDesiredState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get desired state", "resource", "service")

	// Lookup the desired state of the service to have a reference of ports how
	// they should be.
	dState := []apiv1.ServicePort{}
	for _, p := range customObject.Spec.ProtocolPorts {
		servicePortName := fmt.Sprintf(
			PortNameFormat,
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

		dState = append(dState, newPort)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found desired state: %#v", dState), "resource", "service")

	return dState, nil
}

func (r *Resource) NewUpdatePatch(ctx context.Context, obj, currentState, desiredState interface{}) (*framework.Patch, error) {
	create, err := r.newCreateChange(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	update, err := r.newUpdateChange(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	patch := framework.NewPatch()
	patch.SetCreateChange(create)
	patch.SetUpdateChange(update)

	return patch, nil
}

func (r *Resource) newCreateChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}
	currentService, err := toService(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.([]apiv1.ServicePort)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", []apiv1.ServicePort{}, desiredState)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get create state", "resource", "service")

	// Make sure the current state of the Kubernetes resources is known by the
	// create action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	createState := currentService

	// Find anything which is in desired state but not in the current state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be created. Note that the creation
	// of config-map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. creation of resources. In our
	// case here we only transform data within resources. Therefore the update.
	for _, p := range dState {
		if !inServicePorts(createState.Spec.Ports, p) {
			createState.Spec.Ports = append(createState.Spec.Ports, p)
		}
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found create state: %#v", createState), "resource", "service")

	return createState, nil
}

func (r *Resource) NewDeletePatch(ctx context.Context, obj, currentState, desiredState interface{}) (*framework.Patch, error) {
	delete, err := r.newDeleteChange(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	patch := framework.NewPatch()
	patch.SetDeleteChange(delete)

	return patch, nil
}

func (r *Resource) newDeleteChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}
	currentService, err := toService(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.([]apiv1.ServicePort)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", []apiv1.ServicePort{}, desiredState)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get delete state", "resource", "service")

	// Make sure the current state of the Kubernetes resources is known by the
	// delete action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	deleteState := currentService

	// Find anything which is in current state but not in the desired state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be deleted. Note that the deletion
	// of config-map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. deletion of resources. In our
	// case here we only transform data within resources. Therefore the update.
	var newPorts []apiv1.ServicePort
	for _, p := range deleteState.Spec.Ports {
		if !inServicePorts(dState, p) {
			newPorts = append(newPorts, p)
		}
	}
	deleteState.Spec.Ports = newPorts

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found delete state: %#v", deleteState), "resource", "service")

	return deleteState, nil
}

func (r *Resource) newUpdateChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	return nil, nil
}

func (r *Resource) Name() string {
	return Name
}

func (r *Resource) ApplyCreateChange(ctx context.Context, obj, createChange interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	serviceToCreate, err := toService(createChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if serviceToCreate != nil {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "creating the service data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().Services(namespace).Update(serviceToCreate)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "created the service data in the Kubernetes API")
	} else {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "the service data does not need to be created in the Kubernetes API")
	}

	return nil
}

func (r *Resource) ApplyDeleteChange(ctx context.Context, obj, deleteChange interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	serviceToDelete, err := toService(deleteChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if serviceToDelete != nil {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "deleting the service data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().Services(namespace).Update(serviceToDelete)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "deleted the service data in the Kubernetes API")
	} else {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "the service data does not need to be deleted in the Kubernetes API")
	}

	return nil
}

func (r *Resource) ApplyUpdateChange(ctx context.Context, obj, updateChange interface{}) error {
	return nil
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

func toCustomObject(v interface{}) (ingresstpr.CustomObject, error) {
	customObjectPointer, ok := v.(*ingresstpr.CustomObject)
	if !ok {
		return ingresstpr.CustomObject{}, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, v)
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
