// Package service implements business logic to create Kubernetes resources
// against the Kubernetes API.
package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/cenk/backoff"
	"github.com/giantswarm/microendpoint/service/version"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/client/k8s"
	"github.com/giantswarm/operatorkit/framework"
	"github.com/giantswarm/operatorkit/framework/metricsresource"
	"github.com/giantswarm/operatorkit/framework/retryresource"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"

	"github.com/giantswarm/ingress-operator/flag"
	"github.com/giantswarm/ingress-operator/service/healthz"
	"github.com/giantswarm/ingress-operator/service/operator"
	"github.com/giantswarm/ingress-operator/service/resource/configmap"
	"github.com/giantswarm/ingress-operator/service/resource/service"
)

// Config represents the configuration used to create a new service.
type Config struct {
	// Dependencies.
	Logger micrologger.Logger

	// Settings.
	Flag  *flag.Flag
	Viper *viper.Viper

	Description string
	GitCommit   string
	Name        string
	Source      string
}

// DefaultConfig provides a default configuration to create a new service by
// best effort.
func DefaultConfig() Config {
	return Config{
		// Dependencies.
		Logger: nil,

		// Settings.
		Flag:  nil,
		Viper: nil,

		Description: "",
		GitCommit:   "",
		Name:        "",
		Source:      "",
	}
}

// New creates a new configured service object.
func New(config Config) (*Service, error) {
	// Dependencies.
	if config.Logger == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Logger must not be empty")
	}
	config.Logger.Log("debug", fmt.Sprintf("creating ingress-operator with config: %#v", config))

	// Settings.
	if config.Flag == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Flag must not be empty")
	}
	if config.Viper == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Viper must not be empty")
	}

	var err error

	var k8sClient kubernetes.Interface
	{
		k8sConfig := k8s.DefaultConfig()

		k8sConfig.Address = config.Viper.GetString(config.Flag.Service.Kubernetes.Address)
		k8sConfig.Logger = config.Logger
		k8sConfig.InCluster = config.Viper.GetBool(config.Flag.Service.Kubernetes.InCluster)
		k8sConfig.TLS.CAFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CAFile)
		k8sConfig.TLS.CrtFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CrtFile)
		k8sConfig.TLS.KeyFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.KeyFile)

		k8sClient, err = k8s.NewClient(k8sConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var operatorFramework *framework.Framework
	{
		frameworkConfig := framework.DefaultConfig()

		frameworkConfig.Logger = config.Logger

		operatorFramework, err = framework.New(frameworkConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var configMapResource *configmap.Service
	{
		operatorConfig := configmap.DefaultConfig()

		operatorConfig.K8sClient = k8sClient
		operatorConfig.Logger = config.Logger

		configMapResource, err = configmap.New(operatorConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var serviceResource *service.Service
	{
		operatorConfig := service.DefaultConfig()

		operatorConfig.K8sClient = k8sClient
		operatorConfig.Logger = config.Logger

		serviceResource, err = service.New(operatorConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	// We create the list of resources and wrap each resource around some common
	// resources like metrics and retry resources.
	//
	// NOTE that the retry resources wrap the underlying resources first. The
	// wrapped resources are then wrapped around the metrics resource. That way
	// the metrics also consider execution times and execution attempts including
	// retries.
	var resources []framework.Resource
	{
		resources = []framework.Resource{
			configMapResource,
			serviceResource,
		}

		retryWrapConfig := retryresource.DefaultWrapConfig()
		retryWrapConfig.BackOffFactory = func() backoff.BackOff { return backoff.NewExponentialBackOff() }
		retryWrapConfig.Logger = config.Logger
		resources, err = retryresource.Wrap(resources, retryWrapConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		metricsWrapConfig := metricsresource.DefaultWrapConfig()
		metricsWrapConfig.Namespace = config.Name
		resources, err = metricsresource.Wrap(resources, metricsWrapConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var healthzService *healthz.Service
	{
		healthzConfig := healthz.DefaultConfig()

		healthzConfig.K8sClient = k8sClient
		healthzConfig.Logger = config.Logger

		healthzService, err = healthz.New(healthzConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var operatorBackOff *backoff.ExponentialBackOff
	{
		operatorBackOff = backoff.NewExponentialBackOff()
		operatorBackOff.MaxElapsedTime = 5 * time.Minute
	}

	var operatorService *operator.Service
	{
		operatorConfig := operator.DefaultConfig()

		operatorConfig.BackOff = operatorBackOff
		operatorConfig.K8sClient = k8sClient
		operatorConfig.Logger = config.Logger
		operatorConfig.OperatorFramework = operatorFramework
		operatorConfig.Resources = resources
		operatorService, err = operator.New(operatorConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var versionService *version.Service
	{
		versionConfig := version.DefaultConfig()

		versionConfig.Description = config.Description
		versionConfig.GitCommit = config.GitCommit
		versionConfig.Name = config.Name
		versionConfig.Source = config.Source

		versionService, err = version.New(versionConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	newService := &Service{
		// Dependencies.
		Healthz:  healthzService,
		Operator: operatorService,
		Version:  versionService,

		// Internals
		bootOnce: sync.Once{},
	}

	return newService, nil
}

type Service struct {
	// Dependencies.
	Healthz  *healthz.Service
	Operator *operator.Service
	Version  *version.Service

	// Internals.
	bootOnce sync.Once
}

func (s *Service) Boot() {
	s.bootOnce.Do(func() {
		s.Operator.Boot()
	})
}
