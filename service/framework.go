package service

import (
	"encoding/json"
	"fmt"

	"github.com/cenkalti/backoff"
	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/apiextensions/pkg/clientset/versioned"
	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/ingresstpr/protocolport"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/micrologger/microloggertest"
	"github.com/giantswarm/operatorkit/client/k8sclient"
	"github.com/giantswarm/operatorkit/client/k8scrdclient"
	"github.com/giantswarm/operatorkit/client/k8sextclient"
	"github.com/giantswarm/operatorkit/framework"
	"github.com/giantswarm/operatorkit/framework/resource/metricsresource"
	"github.com/giantswarm/operatorkit/framework/resource/retryresource"
	"github.com/giantswarm/operatorkit/informer"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/giantswarm/ingress-operator/service/resource/configmapv2"
	"github.com/giantswarm/ingress-operator/service/resource/servicev2"
)

const (
	ResourceRetries uint64 = 3
)

const (
	IngressConfigCleanupFinalizer = "ingress-operator.giantswarm.io/custom-object-cleanup"
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
		operatorConfig := configmapv2.DefaultConfig()

		operatorConfig.K8sClient = k8sClient
		operatorConfig.Logger = config.Logger

		configMapResource, err = configmapv2.New(operatorConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var serviceResource framework.Resource
	{
		operatorConfig := servicev2.DefaultConfig()

		operatorConfig.K8sClient = k8sClient
		operatorConfig.Logger = config.Logger

		serviceResource, err = servicev2.New(operatorConfig)
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

		retryWrapConfig := retryresource.DefaultWrapConfig()
		retryWrapConfig.BackOffFactory = func() backoff.BackOff { return backoff.WithMaxTries(backoff.NewExponentialBackOff(), ResourceRetries) }
		retryWrapConfig.Logger = config.Logger
		resources, err = retryresource.Wrap(resources, retryWrapConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		metricsWrapConfig := metricsresource.DefaultWrapConfig()
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

	// TODO remove after migration.
	migrateTPRsToCRDs(config.Logger, clientSet)

	var newWatcherFactory informer.WatcherFactory
	{
		newWatcherFactory = func() (watch.Interface, error) {
			watcher, err := clientSet.CoreV1alpha1().IngressConfigs("").Watch(apismetav1.ListOptions{})
			if err != nil {
				return nil, microerror.Mask(err)
			}

			return watcher, nil
		}
	}

	var newInformer *informer.Informer
	{
		informerConfig := informer.DefaultConfig()

		informerConfig.WatcherFactory = newWatcherFactory

		newInformer, err = informer.New(informerConfig)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var crdFramework *framework.Framework
	{
		c := framework.DefaultConfig()

		c.CRD = v1alpha1.NewIngressConfigCRD()
		c.CRDClient = crdClient
		c.Informer = newInformer
		c.Logger = config.Logger
		c.ResourceRouter = framework.DefaultResourceRouter(resources)

		crdFramework, err = framework.New(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	return crdFramework, nil
}

func migrateTPRsToCRDs(logger micrologger.Logger, clientSet *versioned.Clientset) {
	logger.Log("debug", "start TPR migration")

	var err error

	// List all TPOs.
	var b []byte
	{
		e := "/apis/giantswarm.io/v1/namespaces/default/ingressconfigs"
		b, err = clientSet.Discovery().RESTClient().Get().AbsPath(e).DoRaw()
		if err != nil {
			logger.Log("error", fmt.Sprintf("%#v", err))
			return
		}

		fmt.Printf("\n")
		fmt.Printf("b start\n")
		fmt.Printf("%s\n", b)
		fmt.Printf("b end\n")
		fmt.Printf("\n")
	}

	// Convert bytes into structure.
	var v *ingresstpr.List
	{
		v = &ingresstpr.List{}
		if err := json.Unmarshal(b, v); err != nil {
			logger.Log("error", fmt.Sprintf("%#v", err))
			return
		}

		fmt.Printf("\n")
		fmt.Printf("v start\n")
		fmt.Printf("%#v\n", v)
		fmt.Printf("v end\n")
		fmt.Printf("\n")
	}

	// Iterate over all TPOs.
	for _, tpo := range v.Items {
		// Compute CRO using TPO.
		var cro *v1alpha1.IngressConfig
		{
			cro = &v1alpha1.IngressConfig{}

			cro.TypeMeta.APIVersion = "core.giantswarm.io"
			cro.TypeMeta.Kind = "IngressConfig"
			cro.ObjectMeta.Name = tpo.Name
			//cro.ObjectMeta.Finalizers = []string{
			//	IngressConfigCleanupFinalizer,
			//}
			cro.Spec.GuestCluster.ID = tpo.Spec.GuestCluster.ID
			cro.Spec.GuestCluster.Namespace = tpo.Spec.GuestCluster.Namespace
			cro.Spec.GuestCluster.Service = tpo.Spec.GuestCluster.Service
			cro.Spec.HostCluster.IngressController.ConfigMap = tpo.Spec.HostCluster.IngressController.ConfigMap
			cro.Spec.HostCluster.IngressController.Namespace = tpo.Spec.HostCluster.IngressController.Namespace
			cro.Spec.HostCluster.IngressController.Service = tpo.Spec.HostCluster.IngressController.Service
			cro.Spec.ProtocolPorts = toProtocolPorts(tpo.Spec.ProtocolPorts)

			fmt.Printf("\n")
			fmt.Printf("cro start\n")
			fmt.Printf("%#v\n", cro)
			fmt.Printf("cro end\n")
			fmt.Printf("\n")
		}

		// Create CRO in Kubernetes API.
		{
			_, err := clientSet.CoreV1alpha1().IngressConfigs(tpo.Namespace).Get(cro.Name, apismetav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				_, err := clientSet.CoreV1alpha1().IngressConfigs(tpo.Namespace).Create(cro)
				if err != nil {
					logger.Log("error", fmt.Sprintf("%#v", err))
					return
				}
			} else if err != nil {
				logger.Log("error", fmt.Sprintf("%#v", err))
				return
			}
		}
	}

	logger.Log("debug", "end TPR migration")
}

func toProtocolPorts(protocolPortList []protocolport.ProtocolPort) []v1alpha1.IngressConfigSpecProtocolPort {
	var newList []v1alpha1.IngressConfigSpecProtocolPort

	for _, port := range protocolPortList {
		p := v1alpha1.IngressConfigSpecProtocolPort{
			IngressPort: port.IngressPort,
			LBPort:      port.LBPort,
			Protocol:    port.Protocol,
		}

		newList = append(newList, p)
	}

	return newList
}
