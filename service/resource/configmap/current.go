package configmap

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

	// Lookup the current state of the configmap.
	namespace := customObject.Spec.HostCluster.IngressController.Namespace
	configMap := customObject.Spec.HostCluster.IngressController.ConfigMap
	k8sConfigMap, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Get(configMap, apismetav1.GetOptions{})
	if err != nil {
		return nil, microerror.Mask(err)
	}
	// Ensure that the map is assignable. This prevents panics down the road in
	// case the config map has no data at all.
	if k8sConfigMap.Data == nil {
		k8sConfigMap.Data = map[string]string{}
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found k8s state: %#v", *k8sConfigMap))

	return k8sConfigMap, nil
}
