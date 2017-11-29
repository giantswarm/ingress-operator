package configmapv1

import (
	"context"
	"fmt"
	"strconv"

	"github.com/giantswarm/microerror"
)

func (r *Resource) GetDesiredState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get desired state")

	// Lookup the desired state of the config map to have a reference of data how
	// it should be.
	dState := map[string]string{}
	for _, p := range customObject.Spec.ProtocolPorts {
		configMapKey := strconv.Itoa(p.LBPort)
		configMapValue := fmt.Sprintf(
			DataValueFormat,
			customObject.Spec.GuestCluster.Namespace,
			customObject.Spec.GuestCluster.Service,
			p.IngressPort,
		)

		dState[configMapKey] = configMapValue
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found desired state: %#v", dState))

	return dState, nil
}
