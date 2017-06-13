package operator

import (
	"testing"

	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func Test_Service_newClusterPort(t *testing.T) {
	testCases := []struct {
		K8sPorts       []apiv1.ServicePort
		AvailablePorts []int
		Expected       int
		ErrorMatcher   func(error) bool
	}{
		{
			K8sPorts: []apiv1.ServicePort{
				{
					Name: "test-port-one",
					Port: 30001,
				},
				{
					Name: "test-port-two",
					Port: 30002,
				},
				{
					Name: "test-port-three",
					Port: 30003,
				},
			},
			AvailablePorts: []int{
				30001,
				30002,
				30003,
				30004,
			},
			Expected:     30004,
			ErrorMatcher: nil,
		},
		{
			K8sPorts: []apiv1.ServicePort{
				{
					Name: "test-port-one",
					Port: 30001,
				},
				{
					Name: "test-port-two",
					Port: 30002,
				},
				{
					Name: "test-port-four",
					Port: 30004,
				},
			},
			AvailablePorts: []int{
				30001,
				30002,
				30003,
				30004,
				30005,
			},
			Expected:     30003,
			ErrorMatcher: nil,
		},
		{
			K8sPorts: []apiv1.ServicePort{
				{
					Name: "test-port-one",
					Port: 30001,
				},
				{
					Name: "test-port-three",
					Port: 30003,
				},
				{
					Name: "test-port-four",
					Port: 30004,
				},
				{
					Name: "test-port-five",
					Port: 30005,
				},
			},
			AvailablePorts: []int{
				30001,
				30002,
				30003,
				30004,
				30005,
				30006,
			},
			Expected:     30002,
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
		expected, err := newService.newClusterPort(testCase.K8sPorts, testCase.AvailablePorts)
		if err != nil && testCase.ErrorMatcher == nil {
			t.Fatal("case", i+1, "expected", nil, "got", err)
		}
		if testCase.ErrorMatcher != nil && !testCase.ErrorMatcher(err) {
			t.Fatal("case", i+1, "expected", true, "got", false)
		}
		if testCase.Expected != expected {
			t.Fatalf("case %d expected %#v got %#v", i+1, expected, testCase.Expected)
		}
	}
}
