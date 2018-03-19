package configmap

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/framework"
	apiv1 "k8s.io/api/core/v1"
)

func (r *Resource) ApplyUpdateChange(ctx context.Context, obj, updateChange interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	configMapToUpdate, err := toConfigMap(updateChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if configMapToUpdate != nil {
		r.logger.LogCtx(ctx, "level", "debug", "message", "updating the config map data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Update(configMapToUpdate)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.LogCtx(ctx, "level", "debug", "message", "updated the config map data in the Kubernetes API")
	} else {
		r.logger.LogCtx(ctx, "level", "debug", "message", "the config map data does not need to be updated from the Kubernetes API")
	}

	return nil
}

func (r *Resource) NewUpdatePatch(ctx context.Context, obj, currentState, desiredState interface{}) (*framework.Patch, error) {
	update, err := r.newUpdateChange(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	patch := framework.NewPatch()
	patch.SetUpdateChange(update)

	return patch, nil
}

func (r *Resource) newUpdateChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	currentConfigMap, err := toConfigMap(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.(map[string]string)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", map[string]string{}, desiredState)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "finding out which config map items have to be updated")

	var updateState *apiv1.ConfigMap
	var count int
	{
		updateState = currentConfigMap

		for k, v := range dState {
			if !inConfigMapData(updateState.Data, k, v) {
				updateState.Data[k] = v
				count++
			}
		}
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("found %d config map items that have to be updated", count))

	return updateState, nil
}
