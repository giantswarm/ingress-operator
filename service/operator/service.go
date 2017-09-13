package operator

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/cenk/backoff"
	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/framework"
	"github.com/giantswarm/operatorkit/tpr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Config represents the configuration used to create a new service.
type Config struct {
	// Dependencies.
	BackOff           backoff.BackOff
	K8sClient         kubernetes.Interface
	Logger            micrologger.Logger
	OperatorFramework *framework.Framework
	Resources         []framework.Resource
}

// DefaultConfig provides a default configuration to create a new service by
// best effort.
func DefaultConfig() Config {
	return Config{
		// Dependencies.
		BackOff:   nil,
		K8sClient: nil,
		Logger:    nil,
		Resources: nil,
	}
}

// Service implements the service.
type Service struct {
	// Dependencies.
	backOff           backoff.BackOff
	logger            micrologger.Logger
	operatorFramework *framework.Framework
	resources         []framework.Resource

	// Internals.
	bootOnce sync.Once
	mutex    sync.Mutex
	tpr      *tpr.TPR
}

// New creates a new configured service.
func New(config Config) (*Service, error) {
	// Dependencies.
	if config.BackOff == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.BackOff must not be empty")
	}
	if config.K8sClient == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.K8sClient must not be empty")
	}
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Logger must not be empty")
	}
	if config.OperatorFramework == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.OperatorFramework must not be empty")
	}
	if len(config.Resources) == 0 {
		return nil, microerror.Maskf(invalidConfigError, "config.Resources must not be empty")
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
			return nil, microerror.Mask(err)
		}
	}

	newService := &Service{
		// Dependencies.
		backOff:           config.BackOff,
		logger:            config.Logger,
		operatorFramework: config.OperatorFramework,
		resources:         config.Resources,

		// Internals
		bootOnce: sync.Once{},
		mutex:    sync.Mutex{},
		tpr:      newTPR,
	}

	return newService, nil
}

// Boot starts the service and implements the watch for the cluster TPR.
func (s *Service) Boot() {
	s.bootOnce.Do(func() {
		o := func() error {
			err := s.bootWithError()
			if err != nil {
				return microerror.Mask(err)
			}

			return nil
		}

		n := func(err error, d time.Duration) {
			s.logger.Log("warning", fmt.Sprintf("retrying operator boot due to error: %#v", microerror.Mask(err)))
		}

		err := backoff.RetryNotify(o, s.backOff, n)
		if err != nil {
			s.logger.Log("error", fmt.Sprintf("stop operator boot retries due to too many errors: %#v", microerror.Mask(err)))
			os.Exit(1)
		}
	})
}

func (s *Service) bootWithError() error {
	err := s.tpr.CreateAndWait()
	if tpr.IsAlreadyExists(err) {
		s.logger.Log("debug", "third party resource already exists")
	} else if err != nil {
		return microerror.Mask(err)
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

	return nil
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

	err := s.operatorFramework.ProcessCreate(obj, s.resources)
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

	err := s.operatorFramework.ProcessDelete(obj, s.resources)
	if err != nil {
		s.logger.Log("error", fmt.Sprintf("%#v", err), "event", "delete")
	}
}
