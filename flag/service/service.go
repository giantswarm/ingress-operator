package service

import (
	"github.com/giantswarm/ingress-operator/flag/service/ingresscontroller"
	"github.com/giantswarm/ingress-operator/flag/service/kubernetes"
)

type Service struct {
	IngressController ingresscontroller.IngressController
	Kubernetes        kubernetes.Kubernetes
}
