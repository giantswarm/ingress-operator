package service

import (
	"context"
	"fmt"

	"github.com/giantswarm/cert-operator/service/controller/v2/key"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/controller/context/finalizerskeptcontext"
	"github.com/giantswarm/operatorkit/controller/context/resourcecanceledcontext"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Resource) GetCurrentState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "get current state")

	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	service := customObject.Spec.HostCluster.IngressController.Service
	k8sService, err := r.k8sClient.CoreV1().Services(namespace).Get(service, apismetav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("found k8s state: %#v", *k8sService))

	// In case a cluster deletion happens, we want to delete the ingress
	// controller service data. We still need to use it for resource creation in
	// order to drain nodes on KVM though. So as long as pods are there we delay
	// the deletion of the service data here in order to still be able to connect
	// to the guest cluster API via ingress. As soon as the draining was done and
	// the pods got removed we get an empty list here after the delete event got
	// replayed. Then we just remove the service data as usual.
	if key.IsInDeletionState(customObject) {
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

	return k8sService, nil
}
