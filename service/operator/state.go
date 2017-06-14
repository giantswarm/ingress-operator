package operator

import (
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

type OperatorState struct {
	ConfigMap ConfigMapState
	Service   ServiceState
}

type ConfigMapState struct {
	ConfigMap apiv1.ConfigMap
	Data      map[string]string
}

type ServiceState struct {
	Ports   []apiv1.ServicePort
	Service apiv1.Service
}
