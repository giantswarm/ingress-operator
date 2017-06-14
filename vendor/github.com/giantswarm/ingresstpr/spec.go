package ingresstpr

import (
	"github.com/giantswarm/ingresstpr/guestcluster"
	"github.com/giantswarm/ingresstpr/hostcluster"
	"github.com/giantswarm/ingresstpr/protocolport"
)

type Spec struct {
	GuestCluster  guestcluster.GuestCluster   `json:"guestcluster" yaml:"guestcluster"`
	HostCluster   hostcluster.HostCluster     `json:"hostcluster" yaml:"hostcluster"`
	ProtocolPorts []protocolport.ProtocolPort `json:"protocolPorts" yaml:"protocolPorts"`
}
