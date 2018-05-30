package v2

import (
	"github.com/giantswarm/versionbundle"
)

func VersionBundle() versionbundle.Bundle {
	return versionbundle.Bundle{
		Changelogs: []versionbundle.Changelog{
			{
				Component:   "ingress-operator",
				Description: "Introduce the first version of the ingress-operator.",
				Kind:        versionbundle.KindAdded,
			},
		},
		Components: []versionbundle.Component{
			{
				Name:    "ingress-operator",
				Version: "0.1.0",
			},
		},
		Name:    "ingress-operator",
		Version: "0.1.0",
	}
}
