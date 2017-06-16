package operator

import (
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

type ActionState struct {
	ConfigMap apiv1.ConfigMap
	Service   apiv1.Service
}

type CurrentState struct {
	ConfigMap apiv1.ConfigMap
	Service   apiv1.Service
}

type DesiredState struct {
	ConfigMapData map[string]string
	ServicePorts  []apiv1.ServicePort
}
