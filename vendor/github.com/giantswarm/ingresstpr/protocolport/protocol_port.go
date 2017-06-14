package protocolport

type ProtocolPort struct {
	IngressPort string `json:"ingressPort" yaml:"ingressPort"`
	LBPort      string `json:"lbPort" yaml:"lbPort"`
	Protocol    string `json:"protocol" yaml:"protocol"`
}
