package hostcluster

import (
	"github.com/giantswarm/ingresstpr/hostcluster/ingresscontroller"
)

type HostCluster struct {
	IngressController ingresscontroller.IngressController `json:"ingressController" yaml:"ingressController"`
}
