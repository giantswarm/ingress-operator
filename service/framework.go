package service

import (
	"context"

	"github.com/cenkalti/backoff"
	"github.com/giantswarm/api/service"
	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/apiextensions/pkg/clientset/versioned"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger/microloggertest"
	"github.com/giantswarm/operatorkit/client/k8sclient"
	"github.com/giantswarm/operatorkit/client/k8scrdclient"
	"github.com/giantswarm/operatorkit/client/k8sextclient"
	"github.com/giantswarm/operatorkit/framework"
	"github.com/giantswarm/operatorkit/framework/resource/metricsresource"
	"github.com/giantswarm/operatorkit/framework/resource/retryresource"
	"github.com/giantswarm/operatorkit/informer"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/giantswarm/ingress-operator/service/ingressconfig/v2/resource/configmap"
)

const (
	ResourceRetries uint64 = 3
)

func newCRDFramework(config Config) (*framework.Framework, error) {
	if config.Flag == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Flag must not be empty")
	}
	if config.Viper == nil {
		return nil, microerror.Maskf(invalidConfigError, "config.Viper must not be empty")
	}

	var err error

	var k8sClient kubernetes.Interface
	{
		c := k8sclient.DefaultConfig()

		c.Logger = config.Logger

		c.Address = config.Viper.GetString(config.Flag.Service.Kubernetes.Address)
		c.InCluster = config.Viper.GetBool(config.Flag.Service.Kubernetes.InCluster)
		c.TLS.CAFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CAFile)
		c.TLS.CrtFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CrtFile)
		c.TLS.KeyFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.KeyFile)

		k8sClient, err = k8sclient.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var k8sExtClient apiextensionsclient.Interface
	{
		c := k8sextclient.DefaultConfig()

		c.Logger = config.Logger

		c.Address = config.Viper.GetString(config.Flag.Service.Kubernetes.Address)
		c.InCluster = config.Viper.GetBool(config.Flag.Service.Kubernetes.InCluster)
		c.TLS.CAFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CAFile)
		c.TLS.CrtFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CrtFile)
		c.TLS.KeyFile = config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.KeyFile)

		k8sExtClient, err = k8sextclient.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var crdClient *k8scrdclient.CRDClient
	{
		c := k8scrdclient.DefaultConfig()

		c.K8sExtClient = k8sExtClient
		c.Logger = microloggertest.New()

		crdClient, err = k8scrdclient.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var configMapResource framework.Resource
	{
		operatorConfig := configmap.DefaultConfig()

		operatorConfig.K8sClient = k8sClient
		operatorConfig.Logger = config.Logger

		configMapResource, err = configmap.New(operatorConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var serviceResource framework.Resource
	{
		operatorConfig := service.DefaultConfig()

		operatorConfig.K8sClient = k8sClient
		operatorConfig.Logger = config.Logger

		serviceResource, err = service.New(operatorConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var resources []framework.Resource
	{
		resources = []framework.Resource{
			configMapResource,
			serviceResource,
		}

		retryWrapConfig := retryresource.WrapConfig{}
		retryWrapConfig.BackOffFactory = func() backoff.BackOff { return backoff.WithMaxTries(backoff.NewExponentialBackOff(), ResourceRetries) }
		retryWrapConfig.Logger = config.Logger
		resources, err = retryresource.Wrap(resources, retryWrapConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		metricsWrapConfig := metricsresource.WrapConfig{}
		metricsWrapConfig.Name = config.Name
		resources, err = metricsresource.Wrap(resources, metricsWrapConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var clientSet *versioned.Clientset
	{
		var c *rest.Config

		if config.Viper.GetBool(config.Flag.Service.Kubernetes.InCluster) {
			config.Logger.Log("debug", "creating in-cluster config")

			c, err = rest.InClusterConfig()
			if err != nil {
				return nil, microerror.Mask(err)
			}
		} else {
			config.Logger.Log("debug", "creating out-cluster config")

			c = &rest.Config{
				Host: config.Viper.GetString(config.Flag.Service.Kubernetes.Address),
				TLSClientConfig: rest.TLSClientConfig{
					CAFile:   config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CAFile),
					CertFile: config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.CrtFile),
					KeyFile:  config.Viper.GetString(config.Flag.Service.Kubernetes.TLS.KeyFile),
				},
			}
		}

		clientSet, err = versioned.NewForConfig(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var newInformer *informer.Informer
	{
		informerConfig := informer.DefaultConfig()

		informerConfig.Watcher = clientSet.CoreV1alpha1().IngressConfigs("")

		newInformer, err = informer.New(informerConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	handlesFunc := func(obj interface{}) bool {
		return true
	}

	initCtxFunc := func(ctx context.Context, obj interface{}) (context.Context, error) {
		return ctx, nil
	}

	var v2ResourceSet *framework.ResourceSet
	{
		c := framework.ResourceSetConfig{

			Handles:   handlesFunc,
			InitCtx:   initCtxFunc,
			Logger:    config.Logger,
			Resources: resources,
		}

		v2ResourceSet, err = framework.NewResourceSet(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var resourceRouter *framework.ResourceRouter
	{
		c := framework.ResourceRouterConfig{}

		c.ResourceSets = []*framework.ResourceSet{
			v2ResourceSet,
		}

		resourceRouter, err = framework.NewResourceRouter(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var crdFramework *framework.Framework
	{
		c := framework.Config{
			CRD:            v1alpha1.NewIngressConfigCRD(),
			CRDClient:      crdClient,
			Informer:       newInformer,
			Logger:         config.Logger,
			ResourceRouter: resourceRouter,
		}

		crdFramework, err = framework.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return crdFramework, nil
}
