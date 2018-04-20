package key

import (
	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/microerror"
)

func ClusterID(customObject v1alpha1.IngressConfig) string {
	return customObject.Spec.GuestCluster.ID
}

func ClusterNamespace(customObject v1alpha1.IngressConfig) string {
	return customObject.Spec.GuestCluster.Namespace
}

func IsInDeletionState(customObject v1alpha1.IngressConfig) bool {
	return customObject.GetDeletionTimestamp() != nil
}

func ToCustomObject(v interface{}) (v1alpha1.IngressConfig, error) {
	customObjectPointer, ok := v.(*v1alpha1.IngressConfig)
	if !ok {
		return v1alpha1.IngressConfig{}, microerror.Maskf(wrongTypeError, "expected '%T', got '%T'", &v1alpha1.IngressConfig{}, v)
	}
	customObject := *customObjectPointer

	return customObject, nil
}

func VersionBundleVersion(customObject v1alpha1.IngressConfig) string {
	return customObject.Spec.VersionBundle.Version
}
