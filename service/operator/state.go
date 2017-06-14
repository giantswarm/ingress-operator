package operator

import (
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

type OperatorState struct {
	ConfigMap ConfigMapState
	Service   ServiceState
}

type ConfigMapState struct {
	Values map[string]string
}

type ServiceState struct {
	Ports []apiv1.ServicePort
}
