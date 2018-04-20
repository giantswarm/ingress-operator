package service

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/controller"
	apiv1 "k8s.io/api/core/v1"
)

func (r *Resource) ApplyDeleteChange(ctx context.Context, obj, deleteChange interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	serviceToDelete, err := toService(deleteChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if serviceToDelete != nil {
		r.logger.LogCtx(ctx, "level", "debug", "message", "deleting the service data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().Services(namespace).Update(serviceToDelete)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.LogCtx(ctx, "level", "debug", "message", "deleted the service data in the Kubernetes API")
	} else {
		r.logger.LogCtx(ctx, "level", "debug", "message", "the service data does not need to be deleted in the Kubernetes API")
	}

	return nil
}

func (r *Resource) NewDeletePatch(ctx context.Context, obj, currentState, desiredState interface{}) (*controller.Patch, error) {
	delete, err := r.newDeleteChange(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	patch := controller.NewPatch()
	patch.SetDeleteChange(delete)

	return patch, nil
}

func (r *Resource) newDeleteChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	currentService, err := toService(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.([]apiv1.ServicePort)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", []apiv1.ServicePort{}, desiredState)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "get delete state")

	// Make sure the current state of the Kubernetes resources is known by the
	// delete action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	deleteState := currentService

	// Find anything which is in current state but not in the desired state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be deleted. Note that the deletion
	// of config-map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. deletion of resources. In our
	// case here we only transform data within resources. Therefore the update.
	var newPorts []apiv1.ServicePort
	for _, p := range deleteState.Spec.Ports {
		if !inServicePorts(dState, p) {
			newPorts = append(newPorts, p)
		}
	}
	deleteState.Spec.Ports = newPorts

	r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("found delete state: %#v", deleteState))

	return deleteState, nil
}
