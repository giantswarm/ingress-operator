package ingresscontroller

// IngressController holds ingress controller specific information.
type IngressController struct {
	// ConfigMap is the name of the tcp-service config-map resource within a host
	// cluster Kubernetes. This config-map is used to add information about TCP
	// services which the ingress controller running on the host cluster should
	// serve.
	ConfigMap string `json:"configMap" yaml:"configMap"`
	// Namespace is the Kubernetes namespace the ingress controller tcp-services
	// config-map and the ingress controller service is defined in.
	Namespace string `json:"namespace" yaml:"namespace"`
	// Service is the ingress controller service resource within a host cluster
	// Kubernetes.
	Service string `json:"service" yaml:"service"`
}
