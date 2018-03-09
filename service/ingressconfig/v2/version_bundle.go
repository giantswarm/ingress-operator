package v2

import (
	"time"

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
		Dependencies: []versionbundle.Dependency{
			{
				Name:    "kubernetes",
				Version: ">= 1.8.4",
			},
		},
		Deprecated: false,
		Name:       "ingress-operator",
		Time:       time.Date(2018, time.March, 9, 17, 30, 0, 0, time.UTC),
		Version:    "0.1.0",
		WIP:        true,
	}
}
