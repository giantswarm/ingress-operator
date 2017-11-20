package service

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/framework"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func (r *Resource) ApplyUpdateChange(ctx context.Context, obj, updateChange interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	serviceToUpdate, err := toService(updateChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if serviceToUpdate != nil {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "updating the service data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().Services(namespace).Update(serviceToUpdate)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "updated the service data in the Kubernetes API")
	} else {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "the service data does not need to be updated in the Kubernetes API")
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
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}
	currentService, err := toService(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	dState, ok := desiredState.([]apiv1.ServicePort)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", []apiv1.ServicePort{}, desiredState)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get update state", "resource", "service")

	var updateState *apiv1.Service
	{
		updateState = currentService

		for _, p := range dState {
			if !inServicePorts(updateState.Spec.Ports, p) {
				updateState.Spec.Ports = append(updateState.Spec.Ports, p)
			}
		}
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found update state: %#v", updateState), "resource", "service")

	return updateState, nil
}
