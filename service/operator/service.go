package operator

import (
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
)

const (
	PortNameFormat = "ingress-controller-port-%s"
)

// Config represents the configuration used to create a new service.
type Config struct {
	// Dependencies.
	K8sClient kubernetes.Interface
	Logger    micrologger.Logger

	// Settings.
	Namespace string
	Service   string
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
		Namespace: "default",
		Service:   "ingress-controller",
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
	if config.Namespace == "" {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.Namespace must not be empty")
	}
	if config.Service == "" {
		return nil, microerror.MaskAnyf(invalidConfigError, "config.Service must not be empty")
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
		bootOnce:  sync.Once{},
		mutex:     sync.Mutex{},
		namespace: config.Namespace,
		service:   config.Service,
		tpr:       newTPR,
	}

	return newService, nil
}

// Service implements the service.
type Service struct {
	// Dependencies.
	k8sClient kubernetes.Interface
	logger    micrologger.Logger

	// Internals.
	ipsPorts  map[string]int
	bootOnce  sync.Once
	mutex     sync.Mutex
	namespace string
	service   string
	tpr       *tpr.TPR
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

	var err error

	err = s.addPortToService(*customObject)
	if err != nil {
		return microerror.MaskAny(err)
	}

	return nil
}

// addPortToService adds a cluster port to the Kubernetes service resource of
// the configured ingress controller, if it does not already exists.
func (s *Service) addPortToService(customObject kvmtpr.CustomObject) error {
	k8sService, err := s.k8sClient.CoreV1().Services(s.namespace).Get(s.service)
	if err != nil {
		return microerror.MaskAny(err)
	}

	portName := portName(customObject)

	exists := portNameExists(k8sService.Spec.Ports, portName)
	if exists {
		s.logger.Log("debug", fmt.Sprintf("port for cluster '%s' already exists", ClusterID(customObject)))
		return nil
	}

	clusterPort, err := s.newClusterPort(k8sService.Spec.Ports, ipsPortsToPorts(s.ipsPorts))
	if err != nil {
		return microerror.MaskAny(err)
	}

	newPort := apiv1.ServicePort{
		Name:       portName,
		Protocol:   apiv1.ProtocolTCP,
		Port:       int32(clusterPort),
		TargetPort: intstr.FromInt(clusterPort),
		NodePort:   int32(clusterPort),
	}

	k8sService.Spec.Ports = append(k8sService.Spec.Ports, newPort)

	_, err = s.k8sClient.CoreV1().Services(s.namespace).Update(k8sService)
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

func (s *Service) newClusterPort(k8sPorts []apiv1.ServicePort, availablePorts []int) (int, error) {
	for _, a := range availablePorts {
		if portNumberExists(k8sPorts, a) {
			continue
		}

		return a, nil
	}

	return 0, microerror.MaskAnyf(capacityReachedError, "no more ports available")
}

func ipsPortsToPorts(ipsPorts map[string]int) []int {
	var list []int

	for _, v := range ipsPorts {
		list = append(list, v)
	}

	return list
}

func portNameExists(ports []apiv1.ServicePort, name string) bool {
	for _, p := range ports {
		if p.Name == name {
			return true
		}
	}

	return false
}

func portNumberExists(ports []apiv1.ServicePort, number int) bool {
	for _, p := range ports {
		if p.Port == int32(number) {
			return true
		}
	}

	return false
}

func portName(customObject kvmtpr.CustomObject) string {
	return fmt.Sprintf(PortNameFormat, ClusterID(customObject))
}
