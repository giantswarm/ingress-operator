package servicev1

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Resource) GetCurrentState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get current state")

	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	service := customObject.Spec.HostCluster.IngressController.Service
	k8sService, err := r.k8sClient.CoreV1().Services(namespace).Get(service, apismetav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found k8s state: %#v", *k8sService))

	return k8sService, nil
}
