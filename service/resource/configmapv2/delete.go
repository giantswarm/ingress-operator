package configmapv2

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/framework"
)

func (r *Resource) ApplyDeleteChange(ctx context.Context, obj, deleteChange interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	configMapToDelete, err := toConfigMap(deleteChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if configMapToDelete != nil {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "deleting the config map data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Update(configMapToDelete)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "deleted the config map data in the Kubernetes API")
	} else {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "the config map data does not need to be deleted in the Kubernetes API")
	}

	return nil
}

func (r *Resource) NewDeletePatch(ctx context.Context, obj, currentState, desiredState interface{}) (*framework.Patch, error) {
	delete, err := r.newDeleteChange(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	patch := framework.NewPatch()
	patch.SetDeleteChange(delete)

	return patch, nil
}

func (r *Resource) newDeleteChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}
	currentConfigMap, err := toConfigMap(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.(map[string]string)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", map[string]string{}, desiredState)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get delete state")

	// Make sure the current state of the Kubernetes resources is known by the
	// delete action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	deleteState := currentConfigMap

	// Find anything which is in current state but not in the desired state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be deleted. Note that the deletion
	// of config map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. deletion of resources. In our
	// case here we only transform data within resources. Therefore the update.
	newData := map[string]string{}
	for k, v := range deleteState.Data {
		if !inConfigMapData(dState, k, v) {
			newData[k] = v
		}
	}
	deleteState.Data = newData

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found delete state: %#v", deleteState))

	return deleteState, nil
}
