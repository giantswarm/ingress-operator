package configmapv2

import (
	"context"
	"reflect"
	"testing"

	"github.com/giantswarm/apiextensions/pkg/apis/core/v1alpha1"
	"github.com/giantswarm/micrologger/microloggertest"
	"k8s.io/client-go/kubernetes/fake"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func Test_Service_newUpdateChange(t *testing.T) {
	testCases := []struct {
		Obj          interface{}
		CurrentState interface{}
		DesiredState interface{}
		Expected     *apiv1.ConfigMap
		ErrorMatcher func(error) bool
	}{
		// Test 0.
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
			CurrentState: &apiv1.ConfigMap{
				Data: map[string]string{
					"31000": "al9qy/worker:30010",
				},
			},
			DesiredState: map[string]string{
				"31000": "al9qy/worker:30010",
				"31001": "al9qy/worker:30011",
			},
			Expected: &apiv1.ConfigMap{
				Data: map[string]string{
					"31000": "al9qy/worker:30010",
					"31001": "al9qy/worker:30011",
				},
			},
			ErrorMatcher: nil,
		},

		// Test 1.
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
			CurrentState: &apiv1.ConfigMap{
				Data: map[string]string{
					"31000": "p1l6x/worker:30010",
					"31001": "p1l6x/worker:30011",
				},
			},
			DesiredState: map[string]string{
				"31000": "p1l6x/worker:30010",
				"31001": "p1l6x/worker:30011",
				"31002": "p1l6x/worker:30012",
			},
			Expected: &apiv1.ConfigMap{
				Data: map[string]string{
					"31000": "p1l6x/worker:30010",
					"31001": "p1l6x/worker:30011",
					"31002": "p1l6x/worker:30012",
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
		e, ok := result.(*apiv1.ConfigMap)
		if !ok {
			t.Fatalf("test %d expected %#v got %#v", i, true, false)
		}
		if !reflect.DeepEqual(tc.Expected, e) {
			t.Fatalf("test %d expected %#v got %#v", i, tc.Expected, e)
		}
	}
}
