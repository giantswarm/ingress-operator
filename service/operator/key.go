package operator

import (
	"fmt"

	"github.com/giantswarm/kvmtpr"
)

func ClusterID(customObject kvmtpr.CustomObject) string {
	return customObject.Spec.Cluster.Cluster.ID
}

func ClusterNamespace(customObject kvmtpr.CustomObject) string {
	return ClusterID(customObject)
}

func PortName(customObject kvmtpr.CustomObject) string {
	return fmt.Sprintf(PortNameFormat, ClusterID(customObject))
}
