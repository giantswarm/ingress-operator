package operator

import (
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func configmapToPorts(configmap apiv1.ConfigMap) []int {
	var list []int

	for k, v := range configmap.Data {
		list = append(list, int(p))
	}

	return list
}

func serviceToPorts(service apiv1.Service) []int {
	var list []int

	for _, p := range service.Spec.Ports {
		list = append(list, int(p))
	}

	return list
}

func serviceToPortByName(service apiv1.Service, name string) (apiv1.ServicePort, bool) {
	for _, p := range service.Spec.Ports {
		if p.Name == name {
			return p, true
		}
	}

	return apiv1.ServicePort{}, false
}
