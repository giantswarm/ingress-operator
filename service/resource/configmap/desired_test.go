package configmap

import (
	"context"
	"reflect"
	"testing"

	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/ingresstpr/guestcluster"
	"github.com/giantswarm/ingresstpr/hostcluster"
	"github.com/giantswarm/ingresstpr/hostcluster/ingresscontroller"
	"github.com/giantswarm/ingresstpr/protocolport"
	"github.com/giantswarm/micrologger/microloggertest"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_Service_GetDesiredState(t *testing.T) {
	testCases := []struct {
		Obj          interface{}
		Expected     map[string]string
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
			Expected: map[string]string{
				"31000": "al9qy/worker:30010",
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
			Expected: map[string]string{
				"31000": "p1l6x/worker:30010",
				"31001": "p1l6x/worker:30011",
				"31002": "p1l6x/worker:30012",
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

	for i, testCase := range testCases {
		result, err := newResource.GetDesiredState(context.TODO(), testCase.Obj)
		if err != nil && testCase.ErrorMatcher == nil {
			t.Fatal("case", i+1, "expected", nil, "got", err)
		}
		if testCase.ErrorMatcher != nil && !testCase.ErrorMatcher(err) {
			t.Fatal("case", i+1, "expected", true, "got", false)
		}
		e, ok := result.(map[string]string)
		if !ok {
			t.Fatalf("case %d expected %#v got %#v", i+1, true, false)
		}
		if !reflect.DeepEqual(testCase.Expected, e) {
			t.Fatalf("case %d expected %#v got %#v", i+1, testCase.Expected, e)
		}
	}
}