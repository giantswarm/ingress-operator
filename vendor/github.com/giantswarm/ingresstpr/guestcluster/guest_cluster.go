package guestcluster

type GuestCluster struct {
	ID        string `json:"id" yaml:"id"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Service   string `json:"service" yaml:"service"`
}
