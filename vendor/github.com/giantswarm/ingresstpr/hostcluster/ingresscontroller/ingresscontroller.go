package ingresscontroller

type IngressController struct {
	ConfigMap string `json:"configMap" yaml:"configMap"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Service   string `json:"service" yaml:"service"`
}
