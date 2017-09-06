package configmap

import (
	"reflect"
	"testing"

	"github.com/giantswarm/ingress-operator/tests"
	"github.com/giantswarm/ingresstpr"
	"github.com/giantswarm/ingresstpr/guestcluster"
	"github.com/giantswarm/ingresstpr/hostcluster"
	"github.com/giantswarm/ingresstpr/hostcluster/ingresscontroller"
	"github.com/giantswarm/ingresstpr/protocolport"
	"github.com/giantswarm/micrologger/microloggertest"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func testConfig(t *testing.T) Config {
	k8sClient := tests.K8sClient(t)
	newLogger := microloggertest.New()

	return Config{
		K8sClient: k8sClient,
		Logger:    newLogger,
	}
}

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
	var newService *Service
	{
		newConfig := testConfig(t)
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
		e, ok := result.(map[string]string)
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
		Expected     *apiv1.ConfigMap
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
	var newService *Service
	{
		newConfig := testConfig(t)
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
		e, ok := result.(*apiv1.ConfigMap)
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
		Expected     *apiv1.ConfigMap
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
			CurrentState: &apiv1.ConfigMap{
				Data: map[string]string{
					"31000": "al9qy/worker:30010",
					"31001": "al9qy/worker:30011",
				},
			},
			DesiredState: map[string]string{
				"31000": "al9qy/worker:30010",
			},
			Expected: &apiv1.ConfigMap{
				Data: map[string]string{
					"31001": "al9qy/worker:30011",
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
			CurrentState: &apiv1.ConfigMap{
				Data: map[string]string{
					"31000": "p1l6x/worker:30010",
					"31001": "p1l6x/worker:30011",
					"31002": "p1l6x/worker:30012",
				},
			},
			DesiredState: map[string]string{
				"31000": "p1l6x/worker:30010",
				"31001": "p1l6x/worker:30011",
			},
			Expected: &apiv1.ConfigMap{
				Data: map[string]string{
					"31002": "p1l6x/worker:30012",
				},
			},
			ErrorMatcher: nil,
		},
	}

	var err error
	var newService *Service
	{
		newConfig := testConfig(t)
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
		e, ok := result.(*apiv1.ConfigMap)
		if !ok {
			t.Fatalf("case %d expected %#v got %#v", i+1, true, false)
		}
		if !reflect.DeepEqual(testCase.Expected, e) {
			t.Fatalf("case %d expected %#v got %#v", i+1, testCase.Expected, e)
		}
	}
}
