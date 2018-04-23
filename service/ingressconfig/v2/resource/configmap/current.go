package configmap

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/controller/context/finalizerskeptcontext"
	"github.com/giantswarm/operatorkit/controller/context/resourcecanceledcontext"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/ingress-operator/service/ingressconfig/v2/key"
)

func (r *Resource) GetCurrentState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "get current state")

	// Lookup the current state of the configmap.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	configMap := customObject.Spec.HostCluster.IngressController.ConfigMap
	k8sConfigMap, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap, metav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}
	// Ensure that the map is assignable. This prevents panics down the road in
	// case the config map has no data at all.
	if k8sConfigMap.Data == nil {
		k8sConfigMap.Data = map[string]string{}
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("found k8s state: %#v", *k8sConfigMap))

	// In case a cluster deletion happens, we want to delete the ingress
	// controller config map data. We still need to use it for resource creation
	// in order to drain nodes on KVM though. So as long as pods are there we
	// delay the deletion of the config map data here in order to still be able to
	// connect to the guest cluster API via ingress. As soon as the draining was
	// done and the pods got removed we get an empty list here after the delete
	// event got replayed. Then we just remove the config map data as usual.
	if key.IsDeleted(customObject) {
		n := key.ClusterNamespace(customObject)
		list, err := r.k8sClient.CoreV1().Pods(n).List(metav1.ListOptions{})
		if err != nil {
			return nil, microerror.Mask(err)
		}
		if len(list.Items) != 0 {
			r.logger.LogCtx(ctx, "level", "debug", "message", "cannot finish deletion of namespace due to existing pods")
			resourcecanceledcontext.SetCanceled(ctx)
			finalizerskeptcontext.SetKept(ctx)
			r.logger.LogCtx(ctx, "level", "debug", "message", "canceling resource for custom object")

			return nil, nil
		}
	}

	return k8sConfigMap, nil
}
