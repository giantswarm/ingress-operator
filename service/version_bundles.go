package service

import (
	"github.com/giantswarm/versionbundle"

	"github.com/giantswarm/ingress-operator/service/ingressconfig/v2"
)

func NewVersionBundles() []versionbundle.Bundle {
	var versionBundles []versionbundle.Bundle

	versionBundles = append(versionBundles, v2.VersionBundle())

	return versionBundles
}