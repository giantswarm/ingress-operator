package guestcluster

import (
	"github.com/giantswarm/ingress-operator/flag/service/guestcluster/ingresscontroller"
)

type GuestCluster struct {
	IngressController ingresscontroller.IngressController
	Service           string
}
