package guestcluster

// GuestCluster holds guest cluster specific information.
type GuestCluster struct {
	// ID is the guest cluster ID as given by the Giant Swarm platform.
	ID string `json:"id" yaml:"id"`
	// Namespace is the namespace the guest cluster is running in.
	Namespace string `json:"namespace" yaml:"namespace"`
	// Service is the name of the service inside a guest cluster namespace,
	// pointing to the ingress controllers running inside a guest cluster.
	Service string `json:"service" yaml:"service"`
}
