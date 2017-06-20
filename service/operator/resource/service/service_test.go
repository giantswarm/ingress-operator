package service

import (
	"reflect"
	"testing"

	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/ingresstpr/guestcluster"
	"github.com/giantswarm/ingresstpr/hostcluster"
	"github.com/giantswarm/ingresstpr/hostcluster/ingresscontroller"
	"github.com/giantswarm/ingresstpr/protocolport"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/intstr"
)

func Test_Service_GetDesiredState(t *testing.T) {
	testCases := []struct {
		Obj          interface{}
		Expected     DesiredState
		ErrorMatcher func(error) bool
	}{
		{
			Obj: &ingresstpr.CustomObject{
				Spec: ingresstpr.Spec{
					GuestCluster: guestcluster.GuestCluster{
						ID:        "al9qy",
						Namespace: "al9qy",
						Service:   "worker",
					},
					HostCluster: hostcluster.HostCluster{
						IngressController: ingresscontroller.IngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []protocolport.ProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
					},
				},
			},
			Expected: DesiredState{
				ConfigMapData: map[string]string{
					"31000": "al9qy/worker:30010",
				},
				ServicePorts: []apiv1.ServicePort{
					{
						Name:       "http-30010-al9qy",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31000),
						TargetPort: intstr.FromInt(31000),
						NodePort:   int32(31000),
					},
				},
			},
			ErrorMatcher: nil,
		},
		{
			Obj: &ingresstpr.CustomObject{
				Spec: ingresstpr.Spec{
					GuestCluster: guestcluster.GuestCluster{
						ID:        "p1l6x",
						Namespace: "p1l6x",
						Service:   "worker",
					},
					HostCluster: hostcluster.HostCluster{
						IngressController: ingresscontroller.IngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []protocolport.ProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
						{
							IngressPort: 30011,
							Protocol:    "https",
							LBPort:      31001,
						},
						{
							IngressPort: 30012,
							Protocol:    "udp",
							LBPort:      31002,
						},
					},
				},
			},
			Expected: DesiredState{
				ConfigMapData: map[string]string{
					"31000": "p1l6x/worker:30010",
					"31001": "p1l6x/worker:30011",
					"31002": "p1l6x/worker:30012",
				},
				ServicePorts: []apiv1.ServicePort{
					{
						Name:       "http-30010-p1l6x",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31000),
						TargetPort: intstr.FromInt(31000),
						NodePort:   int32(31000),
					},
					{
						Name:       "https-30011-p1l6x",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31001),
						TargetPort: intstr.FromInt(31001),
						NodePort:   int32(31001),
					},
					{
						Name:       "udp-30012-p1l6x",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31002),
						TargetPort: intstr.FromInt(31002),
						NodePort:   int32(31002),
					},
				},
			},
			ErrorMatcher: nil,
		},
	}

	var err error
	var newService *Service
	{
		newConfig := DefaultConfig()
		newService, err = New(newConfig)
		if err != nil {
			t.Fatal("expected", nil, "got", err)
		}
	}

	for i, testCase := range testCases {
		result, err := newService.GetDesiredState(testCase.Obj)
		if err != nil && testCase.ErrorMatcher == nil {
			t.Fatal("case", i+1, "expected", nil, "got", err)
		}
		if testCase.ErrorMatcher != nil && !testCase.ErrorMatcher(err) {
			t.Fatal("case", i+1, "expected", true, "got", false)
		}
		e, ok := result.(DesiredState)
		if !ok {
			t.Fatalf("case %d expected %#v got %#v", i+1, true, false)
		}
		if !reflect.DeepEqual(testCase.Expected, e) {
			t.Fatalf("case %d expected %#v got %#v", i+1, testCase.Expected, e)
		}
	}
}

func Test_Service_GetCreateState(t *testing.T) {
	testCases := []struct {
		Obj          interface{}
		CurrentState interface{}
		DesiredState interface{}
		Expected     ActionState
		ErrorMatcher func(error) bool
	}{
		// Test case 1.
		{
			Obj: &ingresstpr.CustomObject{
				Spec: ingresstpr.Spec{
					GuestCluster: guestcluster.GuestCluster{
						ID:        "al9qy",
						Namespace: "al9qy",
						Service:   "worker",
					},
					HostCluster: hostcluster.HostCluster{
						IngressController: ingresscontroller.IngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []protocolport.ProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
					},
				},
			},
			CurrentState: CurrentState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31000": "al9qy/worker:30010",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Name:       "http-30010-al9qy",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31000),
								TargetPort: intstr.FromInt(31000),
								NodePort:   int32(31000),
							},
						},
					},
				},
			},
			DesiredState: DesiredState{
				ConfigMapData: map[string]string{
					"31000": "al9qy/worker:30010",
					"31001": "al9qy/worker:30011",
				},
				ServicePorts: []apiv1.ServicePort{
					{
						Name:       "http-30010-al9qy",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31000),
						TargetPort: intstr.FromInt(31000),
						NodePort:   int32(31000),
					},
				},
			},
			Expected: ActionState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31000": "al9qy/worker:30010",
						"31001": "al9qy/worker:30011",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Name:       "http-30010-al9qy",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31000),
								TargetPort: intstr.FromInt(31000),
								NodePort:   int32(31000),
							},
						},
					},
				},
			},
			ErrorMatcher: nil,
		},

		// Test case 2.
		{
			Obj: &ingresstpr.CustomObject{
				Spec: ingresstpr.Spec{
					GuestCluster: guestcluster.GuestCluster{
						ID:        "p1l6x",
						Namespace: "p1l6x",
						Service:   "worker",
					},
					HostCluster: hostcluster.HostCluster{
						IngressController: ingresscontroller.IngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []protocolport.ProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
						{
							IngressPort: 30011,
							Protocol:    "https",
							LBPort:      31001,
						},
						{
							IngressPort: 30012,
							Protocol:    "udp",
							LBPort:      31002,
						},
					},
				},
			},
			CurrentState: CurrentState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31000": "p1l6x/worker:30010",
						"31001": "p1l6x/worker:30011",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Name:       "http-30010-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31000),
								TargetPort: intstr.FromInt(31000),
								NodePort:   int32(31000),
							},
						},
					},
				},
			},
			DesiredState: DesiredState{
				ConfigMapData: map[string]string{
					"31000": "p1l6x/worker:30010",
					"31001": "p1l6x/worker:30011",
					"31002": "p1l6x/worker:30012",
				},
				ServicePorts: []apiv1.ServicePort{
					{
						Name:       "http-30010-p1l6x",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31000),
						TargetPort: intstr.FromInt(31000),
						NodePort:   int32(31000),
					},
					{
						Name:       "https-30011-p1l6x",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31001),
						TargetPort: intstr.FromInt(31001),
						NodePort:   int32(31001),
					},
					{
						Name:       "udp-30012-p1l6x",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31002),
						TargetPort: intstr.FromInt(31002),
						NodePort:   int32(31002),
					},
				},
			},
			Expected: ActionState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31000": "p1l6x/worker:30010",
						"31001": "p1l6x/worker:30011",
						"31002": "p1l6x/worker:30012",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Name:       "http-30010-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31000),
								TargetPort: intstr.FromInt(31000),
								NodePort:   int32(31000),
							},
							{
								Name:       "https-30011-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31001),
								TargetPort: intstr.FromInt(31001),
								NodePort:   int32(31001),
							},
							{
								Name:       "udp-30012-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31002),
								TargetPort: intstr.FromInt(31002),
								NodePort:   int32(31002),
							},
						},
					},
				},
			},
			ErrorMatcher: nil,
		},
	}

	var err error
	var newService *Service
	{
		newConfig := DefaultConfig()
		newService, err = New(newConfig)
		if err != nil {
			t.Fatal("expected", nil, "got", err)
		}
	}

	for i, testCase := range testCases {
		result, err := newService.GetCreateState(testCase.Obj, testCase.CurrentState, testCase.DesiredState)
		if err != nil && testCase.ErrorMatcher == nil {
			t.Fatal("case", i+1, "expected", nil, "got", err)
		}
		if testCase.ErrorMatcher != nil && !testCase.ErrorMatcher(err) {
			t.Fatal("case", i+1, "expected", true, "got", false)
		}
		e, ok := result.(ActionState)
		if !ok {
			t.Fatalf("case %d expected %#v got %#v", i+1, true, false)
		}
		if !reflect.DeepEqual(testCase.Expected, e) {
			t.Fatalf("case %d expected %#v got %#v", i+1, testCase.Expected, e)
		}
	}
}

func Test_Service_GetDeleteState(t *testing.T) {
	testCases := []struct {
		Obj          interface{}
		CurrentState interface{}
		DesiredState interface{}
		Expected     ActionState
		ErrorMatcher func(error) bool
	}{
		// Test case 1.
		{
			Obj: &ingresstpr.CustomObject{
				Spec: ingresstpr.Spec{
					GuestCluster: guestcluster.GuestCluster{
						ID:        "al9qy",
						Namespace: "al9qy",
						Service:   "worker",
					},
					HostCluster: hostcluster.HostCluster{
						IngressController: ingresscontroller.IngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []protocolport.ProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
					},
				},
			},
			CurrentState: CurrentState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31000": "al9qy/worker:30010",
						"31001": "al9qy/worker:30011",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Name:       "http-30010-al9qy",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31000),
								TargetPort: intstr.FromInt(31000),
								NodePort:   int32(31000),
							},
						},
					},
				},
			},
			DesiredState: DesiredState{
				ConfigMapData: map[string]string{
					"31000": "al9qy/worker:30010",
				},
				ServicePorts: []apiv1.ServicePort{
					{
						Name:       "http-30010-al9qy",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31000),
						TargetPort: intstr.FromInt(31000),
						NodePort:   int32(31000),
					},
				},
			},
			Expected: ActionState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31001": "al9qy/worker:30011",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: nil,
					},
				},
			},
			ErrorMatcher: nil,
		},

		// Test case 2.
		{
			Obj: &ingresstpr.CustomObject{
				Spec: ingresstpr.Spec{
					GuestCluster: guestcluster.GuestCluster{
						ID:        "p1l6x",
						Namespace: "p1l6x",
						Service:   "worker",
					},
					HostCluster: hostcluster.HostCluster{
						IngressController: ingresscontroller.IngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []protocolport.ProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
						{
							IngressPort: 30011,
							Protocol:    "https",
							LBPort:      31001,
						},
						{
							IngressPort: 30012,
							Protocol:    "udp",
							LBPort:      31002,
						},
					},
				},
			},
			CurrentState: CurrentState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31000": "p1l6x/worker:30010",
						"31001": "p1l6x/worker:30011",
						"31002": "p1l6x/worker:30012",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Name:       "http-30010-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31000),
								TargetPort: intstr.FromInt(31000),
								NodePort:   int32(31000),
							},
							{
								Name:       "https-30011-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31001),
								TargetPort: intstr.FromInt(31001),
								NodePort:   int32(31001),
							},
							{
								Name:       "udp-30012-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31002),
								TargetPort: intstr.FromInt(31002),
								NodePort:   int32(31002),
							},
						},
					},
				},
			},
			DesiredState: DesiredState{
				ConfigMapData: map[string]string{
					"31000": "p1l6x/worker:30010",
					"31001": "p1l6x/worker:30011",
				},
				ServicePorts: []apiv1.ServicePort{
					{
						Name:       "http-30010-p1l6x",
						Protocol:   apiv1.ProtocolTCP,
						Port:       int32(31000),
						TargetPort: intstr.FromInt(31000),
						NodePort:   int32(31000),
					},
				},
			},
			Expected: ActionState{
				ConfigMap: apiv1.ConfigMap{
					Data: map[string]string{
						"31002": "p1l6x/worker:30012",
					},
				},
				Service: apiv1.Service{
					Spec: apiv1.ServiceSpec{
						Ports: []apiv1.ServicePort{
							{
								Name:       "https-30011-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31001),
								TargetPort: intstr.FromInt(31001),
								NodePort:   int32(31001),
							},
							{
								Name:       "udp-30012-p1l6x",
								Protocol:   apiv1.ProtocolTCP,
								Port:       int32(31002),
								TargetPort: intstr.FromInt(31002),
								NodePort:   int32(31002),
							},
						},
					},
				},
			},
			ErrorMatcher: nil,
		},
	}

	var err error
	var newService *Service
	{
		newConfig := DefaultConfig()
		newService, err = New(newConfig)
		if err != nil {
			t.Fatal("expected", nil, "got", err)
		}
	}

	for i, testCase := range testCases {
		result, err := newService.GetDeleteState(testCase.Obj, testCase.CurrentState, testCase.DesiredState)
		if err != nil && testCase.ErrorMatcher == nil {
			t.Fatal("case", i+1, "expected", nil, "got", err)
		}
		if testCase.ErrorMatcher != nil && !testCase.ErrorMatcher(err) {
			t.Fatal("case", i+1, "expected", true, "got", false)
		}
		e, ok := result.(ActionState)
		if !ok {
			t.Fatalf("case %d expected %#v got %#v", i+1, true, false)
		}
		if !reflect.DeepEqual(testCase.Expected, e) {
			t.Fatalf("case %d expected %#v got %#v", i+1, testCase.Expected, e)
		}
	}
}
