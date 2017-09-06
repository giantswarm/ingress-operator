package protocolport

// ProtocolPort describes port relationships between ingress controller specific
// ports and load balancer specific ports.
type ProtocolPort struct {
	// IngressPort describes an ingress controller specific port.
	IngressPort int `json:"ingressPort" yaml:"ingressPort"`
	// LBPort describes an load balancer specific port.
	LBPort int `json:"lbPort" yaml:"lbPort"`
	// Protocol identifies which kind of ingress controller port is going to be
	// mapped. This information is only used to create human readable references
	// of Kubernetes service port names.
	Protocol string `json:"protocol" yaml:"protocol"`
}
