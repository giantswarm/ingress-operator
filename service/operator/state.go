package operator

import (
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

type OperatorState struct {
	ConfigMap ConfigMapState
	Service   ServiceState
}

type ConfigMapState struct {
	Data     map[string]string
	Resource apiv1.ConfigMap
}

type ServiceState struct {
	Ports    []apiv1.ServicePort
	Resource apiv1.Service
}

type ActionState struct {
	ConfigMap apiv1.ConfigMap
	Service   apiv1.Service
}
