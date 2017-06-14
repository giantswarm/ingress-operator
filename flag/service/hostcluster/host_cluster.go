package hostcluster

import (
	"github.com/giantswarm/ingress-operator/flag/service/hostcluster/ingresscontroller"
)

type HostCluster struct {
	AvailablePorts    string
	IngressController ingresscontroller.IngressController
}
