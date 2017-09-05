package ingresstpr

import "github.com/giantswarm/ingresstpr/spec"

type Spec struct {
	// GuestCluster holds information about guest cluster specific settings. This
	// block is used to separate ambiguous settings which we also use in the
	// HostCluster block.
	GuestCluster spec.GuestCluster `json:"guestcluster" yaml:"guestcluster"`
	// HostCluster holds information about host cluster specific settings. This
	// block is used to separate ambiguous settings which we also use in the
	// GuestCluster block.
	HostCluster spec.HostCluster `json:"hostcluster" yaml:"hostcluster"`
	// ProtocolPorts is a list of structures describing port relationships.
	ProtocolPorts []spec.ProtocolPort `json:"protocolPorts" yaml:"protocolPorts"`
}
