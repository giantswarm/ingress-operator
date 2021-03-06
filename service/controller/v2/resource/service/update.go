package service

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/operatorkit/controller"
	apiv1 "k8s.io/api/core/v1"
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
		r.logger.LogCtx(ctx, "level", "debug", "message", "updating the service data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().Services(namespace).Update(serviceToUpdate)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.LogCtx(ctx, "level", "debug", "message", "updated the service data in the Kubernetes API")
	} else {
		r.logger.LogCtx(ctx, "level", "debug", "message", "the service data does not need to be updated in the Kubernetes API")
	}

	return nil
}

func (r *Resource) NewUpdatePatch(ctx context.Context, obj, currentState, desiredState interface{}) (*controller.Patch, error) {
	update, err := r.newUpdateChange(ctx, obj, currentState, desiredState)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	patch := controller.NewPatch()
	patch.SetUpdateChange(update)

	return patch, nil
}

func (r *Resource) newUpdateChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
	currentService, err := toService(currentState)
	if err != nil {
		return microerror.Mask(err), nil
	}
	desiredPorts, ok := desiredState.([]apiv1.ServicePort)
	if !ok {
		return nil, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", []apiv1.ServicePort{}, desiredState)
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", "finding out which service ports have to be updated")

	var serviceToUpdate *apiv1.Service
	var count int
	{
		// TODO use DeepCopy to create a copy of the current service to prevent
		// weird side effects as soon as the method it available.

		for _, desiredPort := range desiredPorts {
			currentPort, err := getServicePortByPort(currentService.Spec.Ports, desiredPort.Port)
			if IsServicePortNotFound(err) {
				currentService.Spec.Ports = append(currentService.Spec.Ports, desiredPort)
				count++
				continue
			}

			if currentPort.Name != desiredPort.Name {
				r.logger.LogCtx(ctx, "level", "warning", "message", "found orphaned service port, overwriting it with desired service port")

				for i, cp := range currentService.Spec.Ports {
					if cp.Port == desiredPort.Port {
						currentService.Spec.Ports[i] = desiredPort
						count++
						break
					}
				}
			}
		}

		if count > 0 {
			serviceToUpdate = currentService
		}
	}

	r.logger.LogCtx(ctx, "level", "debug", "message", fmt.Sprintf("found %d service ports that have to be updated", count))

	return serviceToUpdate, nil
}
