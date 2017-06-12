package service

import (
	"github.com/giantswarm/ingress-operator/flag/service/kubernetes"
)

type Service struct {
	Kubernetes kubernetes.Kubernetes
}
