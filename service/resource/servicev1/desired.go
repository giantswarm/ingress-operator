package servicev1

import (
	"context"
	"fmt"

	"github.com/giantswarm/microerror"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func (r *Resource) GetDesiredState(ctx context.Context, obj interface{}) (interface{}, error) {
	customObject, err := toCustomObject(obj)
	if err != nil {
		return microerror.Mask(err), nil
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", "get desired state")

	// Lookup the desired state of the service to have a reference of ports how
	// they should be.
	dState := []apiv1.ServicePort{}
	for _, p := range customObject.Spec.ProtocolPorts {
		servicePortName := fmt.Sprintf(
			PortNameFormat,
			p.Protocol,
			p.IngressPort,
			customObject.Spec.GuestCluster.ID,
		)

		newPort := apiv1.ServicePort{
			Name:       servicePortName,
			Protocol:   apiv1.ProtocolTCP,
			Port:       int32(p.LBPort),
			TargetPort: intstr.FromInt(p.LBPort),
			NodePort:   int32(p.LBPort),
		}

		dState = append(dState, newPort)
	}

	r.logger.Log("cluster", customObject.Spec.GuestCluster.ID, "debug", fmt.Sprintf("found desired state: %#v", dState))

	return dState, nil
}
