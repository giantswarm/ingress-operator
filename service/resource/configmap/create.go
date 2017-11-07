package configmap

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
)

func (r *Resource) ApplyCreateChange(ctx context.Context, obj, createChange interface{}) error {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err)
	}
	configMapToCreate, err := toConfigMap(createChange)
	if err != nil {
		return microerror.Mask(err)
	}

	if configMapToCreate != nil {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "creating the config map data in the Kubernetes API")

		namespace := customObject.Spec.HostCluster.IngressController.Namespace
		_, err := r.k8sClient.CoreV1().ConfigMaps(namespace).Update(configMapToCreate)
		if err != nil {
			return microerror.Mask(err)
		}

		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "created the config map data in the Kubernetes API")
	} else {
		r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "the config map data does not need to be created from the Kubernetes API")
	}

	return nil
}

func (r *Resource) newCreateChange(ctx context.Context, obj, currentState, desiredState interface{}) (interface{}, error) {
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

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get create state", "resource", "config-map")

	// Make sure the current state of the Kubernetes resources is known by the
	// create action. The resources we already fetched represent the source of
	// truth. They have to be used as base to actually update the resources in the
	// next steps.
	createState := currentConfigMap

	// Find anything which is in desired state but not in the current state. This
	// lets us drive the current state towards the desired state, because
	// everything we find here is supposed to be created. Note that the creation
	// of config map data and service ports is always only an update operation
	// against the Kubernetes API. Anyway, this concept here implements how a real
	// reconciliation should drive specific parts of the current state towards the
	// desired state, because a decent reconciliation is not always only an update
	// operation of existing resources, but e.g. creation of resources. In our
	// case here we only transform data within resources. Therefore the update.
	for k, v := range dState {
		if !inConfigMapData(createState.Data, k, v) {
			createState.Data[k] = v
		}
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found create state: %#v", createState), "resource", "config-map")

	return createState, nil
}
