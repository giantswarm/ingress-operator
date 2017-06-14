package service

import (
	"github.com/giantswarm/ingress-operator/flag/service/guestcluster"
	"github.com/giantswarm/ingress-operator/flag/service/hostcluster"
	"github.com/giantswarm/ingress-operator/flag/service/kubernetes"
)

type Service struct {
	GuestCluster guestcluster.GuestCluster
	HostCluster  hostcluster.HostCluster
	Kubernetes   kubernetes.Kubernetes
}
