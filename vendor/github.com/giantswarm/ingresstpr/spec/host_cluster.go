package spec

import "github.com/giantswarm/ingresstpr/spec/hostcluster"

// HostCluster holds host cluster specific information.
type HostCluster struct {
	// IngressController holds ingress controller specific information.
	IngressController hostcluster.IngressController `json:"ingressController" yaml:"ingressController"`
}
