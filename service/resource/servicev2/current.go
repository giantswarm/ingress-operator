package servicev2

import (
	"context"
)

func (r *Resource) GetCurrentState(ctx context.Context, obj interface{}) (interface{}, error) {
	r.logger.LogCtx(ctx, "info", "v2 resource executed")
	return nil, nil
	/*
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
	*/
}
