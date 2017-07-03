package service

import (
	"fmt"

	"github.com/giantswarm/ingresstpr"
	microerror "github.com/giantswarm/microkit/error"
	micrologger "github.com/giantswarm/microkit/logger"
	"github.com/giantswarm/operatorkit/client/k8s"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

const (
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

	newService := &Service{
		// Dependencies.
		k8sClient: config.K8sClient,
		logger:    config.Logger,
	}

	return newService, nil
}

// Service implements the service.
type Service struct {
	// Dependencies.
	k8sClient kubernetes.Interface
	logger    micrologger.Logger
}

func (s *Service) GetCurrentState(obj interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get current state", "resource", "service")

	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	service := customObject.Spec.HostCluster.IngressController.Service
	k8sService, err := s.k8sClient.CoreV1().Services(namespace).Get(service, apismetav1.GetOptions{})
	if err != nil {
		return nil, microerror.MaskAny(err)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found k8s state: %#v", *k8sService), "resource", "service")

	return k8sService, nil
}

func (s *Service) GetDesiredState(obj interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get desired state", "resource", "service")

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

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found desired state: %#v", dState), "resource", "service")

	return dState, nil
}

func (s *Service) GetCreateState(obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := currentState.(*apiv1.Service)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &apiv1.Service{}, currentState)
	}
	dState, ok := desiredState.([]apiv1.ServicePort)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", []apiv1.ServicePort{}, desiredState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get create state", "resource", "service")

	// Make sure the current state of the Kubernetes resources is known by the
	// create action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	createState := cState

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

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found create state: %#v", createState), "resource", "service")

	return createState, nil
}

func (s *Service) GetDeleteState(obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := currentState.(*apiv1.Service)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &apiv1.Service{}, currentState)
	}
	dState, ok := desiredState.([]apiv1.ServicePort)
	if !ok {
		return nil, microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", []apiv1.ServicePort{}, desiredState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get delete state", "resource", "service")

	// Make sure the current state of the Kubernetes resources is known by the
	// delete action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	deleteState := cState

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

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found delete state: %#v", deleteState), "resource", "service")

	return deleteState, nil
}

func (s *Service) ProcessCreateState(obj, createState interface{}) error {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	cState, ok := createState.(*apiv1.Service)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &apiv1.Service{}, createState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "process create state", "resource", "service")

	// Add the service ports by updating the Kubernetes service resource.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	_, err := s.k8sClient.CoreV1().Services(namespace).Update(cState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "processed create state", "resource", "service")

	return nil
}

func (s *Service) ProcessDeleteState(obj, deleteState interface{}) error {
	customObject, ok := obj.(*ingresstpr.CustomObject)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &ingresstpr.CustomObject{}, obj)
	}
	dState, ok := deleteState.(*apiv1.Service)
	if !ok {
		return microerror.MaskAnyf(wrongTypeError, "expected '%T', got '%T'", &apiv1.Service{}, deleteState)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "process delete state", "resource", "service")

	// Add the service ports by updating the Kubernetes service resource.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	_, err := s.k8sClient.CoreV1().Services(namespace).Update(dState)
	if err != nil {
		return microerror.MaskAny(err)
	}

	s.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "processed delete state", "resource", "service")

	return nil
}

func inServicePorts(ports []apiv1.ServicePort, p apiv1.ServicePort) bool {
	for _, pp := range ports {
		if pp.String() == p.String() {
			return true
		}
	}

	return false
}
