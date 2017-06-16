package hostcluster

import (
	"github.com/giantswarm/ingresstpr/hostcluster/ingresscontroller"
)

// HostCluster holds host cluster specific information.
type HostCluster struct {
	// IngressController holds ingress controller specific information.
	IngressController ingresscontroller.IngressController `json:"ingressController" yaml:"ingressController"`
}
