package protocolport

type ProtocolPort struct {
	IngressPort int    `json:"ingressPort" yaml:"ingressPort"`
	LBPort      int    `json:"lbPort" yaml:"lbPort"`
	Protocol    string `json:"protocol" yaml:"protocol"`
}
