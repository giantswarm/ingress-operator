package servicev2

import (
	"context"
	"reflect"
	"testing"

	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/micrologger/microloggertest"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func Test_Service_newUpdateChange(t *testing.T) {
	testCases := []struct {
		Obj          interface{}
		CurrentState interface{}
		DesiredState interface{}
		Expected     *apiv1.Service
		ErrorMatcher func(error) bool
	}{
		// Test 0 ensures that having one port in the current state and having the
		// same port in the desired state, the update state should be empty.
		{
			Obj: &v1alpha1.IngressConfig{
				Spec: v1alpha1.IngressConfigSpec{
					GuestCluster: v1alpha1.IngressConfigSpecGuestCluster{
						ID:        "al9qy",
						Namespace: "al9qy",
						Service:   "worker",
					},
					HostCluster: v1alpha1.IngressConfigSpecHostCluster{
						IngressController: v1alpha1.IngressConfigSpecHostClusterIngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []v1alpha1.IngressConfigSpecProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
					},
				},
			},
			CurrentState: &apiv1.Service{
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
			DesiredState: []apiv1.ServicePort{
				{
					Name:       "http-30010-al9qy",
					Protocol:   apiv1.ProtocolTCP,
					Port:       int32(31000),
					TargetPort: intstr.FromInt(31000),
					NodePort:   int32(31000),
				},
			},
			Expected:     nil,
			ErrorMatcher: nil,
		},

		// Test 1 is the same as 0 but with multiple ports.
		{
			Obj: &v1alpha1.IngressConfig{
				Spec: v1alpha1.IngressConfigSpec{
					GuestCluster: v1alpha1.IngressConfigSpecGuestCluster{
						ID:        "p1l6x",
						Namespace: "p1l6x",
						Service:   "worker",
					},
					HostCluster: v1alpha1.IngressConfigSpecHostCluster{
						IngressController: v1alpha1.IngressConfigSpecHostClusterIngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []v1alpha1.IngressConfigSpecProtocolPort{
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
			CurrentState: &apiv1.Service{
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
			DesiredState: []apiv1.ServicePort{
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
			Expected:     nil,
			ErrorMatcher: nil,
		},

		// Test 2 ensures that an empty current state causes the port of the desired
		// state to be added to the update state.
		{
			Obj: &v1alpha1.IngressConfig{
				Spec: v1alpha1.IngressConfigSpec{
					GuestCluster: v1alpha1.IngressConfigSpecGuestCluster{
						ID:        "al9qy",
						Namespace: "al9qy",
						Service:   "worker",
					},
					HostCluster: v1alpha1.IngressConfigSpecHostCluster{
						IngressController: v1alpha1.IngressConfigSpecHostClusterIngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []v1alpha1.IngressConfigSpecProtocolPort{
						{
							IngressPort: 30010,
							Protocol:    "http",
							LBPort:      31000,
						},
					},
				},
			},
			CurrentState: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{},
				},
			},
			DesiredState: []apiv1.ServicePort{
				{
					Name:       "http-30010-al9qy",
					Protocol:   apiv1.ProtocolTCP,
					Port:       int32(31000),
					TargetPort: intstr.FromInt(31000),
					NodePort:   int32(31000),
				},
			},
			Expected: &apiv1.Service{
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
			ErrorMatcher: nil,
		},

		// Test 3 ensures that when having one port in the current state and having
		// two new ports in the desired state, the update state contains a
		// composition of these three ports.
		{
			Obj: &v1alpha1.IngressConfig{
				Spec: v1alpha1.IngressConfigSpec{
					GuestCluster: v1alpha1.IngressConfigSpecGuestCluster{
						ID:        "p1l6x",
						Namespace: "p1l6x",
						Service:   "worker",
					},
					HostCluster: v1alpha1.IngressConfigSpecHostCluster{
						IngressController: v1alpha1.IngressConfigSpecHostClusterIngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []v1alpha1.IngressConfigSpecProtocolPort{
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
			CurrentState: &apiv1.Service{
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
			DesiredState: []apiv1.ServicePort{
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
			Expected: &apiv1.Service{
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
			ErrorMatcher: nil,
		},

		// Test 4 ensures overwriting orphaned service ports works as expected.
		{
			Obj: &v1alpha1.IngressConfig{
				Spec: v1alpha1.IngressConfigSpec{
					GuestCluster: v1alpha1.IngressConfigSpecGuestCluster{
						ID:        "p1l6x",
						Namespace: "p1l6x",
						Service:   "worker",
					},
					HostCluster: v1alpha1.IngressConfigSpecHostCluster{
						IngressController: v1alpha1.IngressConfigSpecHostClusterIngressController{
							ConfigMap: "ingress-controller",
							Namespace: "kube-system",
							Service:   "ingress-controller",
						},
					},
					ProtocolPorts: []v1alpha1.IngressConfigSpecProtocolPort{
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
			CurrentState: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "http-30010-foo",
							Protocol:   apiv1.ProtocolTCP,
							Port:       int32(31000),
							TargetPort: intstr.FromInt(31000),
							NodePort:   int32(31000),
						},
						{
							Name:       "https-30011-bar",
							Protocol:   apiv1.ProtocolTCP,
							Port:       int32(31001),
							TargetPort: intstr.FromInt(31001),
							NodePort:   int32(31001),
						},
						{
							Name:       "udp-30012-baz",
							Protocol:   apiv1.ProtocolTCP,
							Port:       int32(31002),
							TargetPort: intstr.FromInt(31002),
							NodePort:   int32(31002),
						},
					},
				},
			},
			DesiredState: []apiv1.ServicePort{
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
			Expected: &apiv1.Service{
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
			ErrorMatcher: nil,
		},
	}

	var err error
	var newResource *Resource
	{
		c := DefaultConfig()

		c.K8sClient = fake.NewSimpleClientset()
		c.Logger = microloggertest.New()

		newResource, err = New(c)
		if err != nil {
			t.Fatal("expected", nil, "got", err)
		}
	}

	for i, tc := range testCases {
		result, err := newResource.newUpdateChange(context.TODO(), tc.Obj, tc.CurrentState, tc.DesiredState)
		if err != nil && tc.ErrorMatcher == nil {
			t.Fatal("test", i, "expected", nil, "got", err)
		}
		if tc.ErrorMatcher != nil && !tc.ErrorMatcher(err) {
			t.Fatal("test", i, "expected", true, "got", false)
		}
		e, ok := result.(*apiv1.Service)
		if !ok {
			t.Fatalf("test %d expected %#v got %#v", i, true, false)
		}
		if !reflect.DeepEqual(tc.Expected, e) {
			t.Fatalf("test %d expected %#v got %#v", i, tc.Expected, e)
		}
	}
}
