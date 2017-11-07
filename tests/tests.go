package tests

import (
	"testing"

	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/client/k8sclient"
	"k8s.io/client-go/kubernetes"
)

func K8sClient(t *testing.T) kubernetes.Interface {
	var err error

	var newLogger micrologger.Logger
	{
		c := micrologger.DefaultConfig()

		newLogger, err = micrologger.New(c)
		if err != nil {
			t.Fatal(err)
		}
	}

	var k8sClient kubernetes.Interface
	{
		c := k8sclient.DefaultConfig()

		c.Address = "http://127.0.0.1:8080"
		c.Logger = newLogger
		c.InCluster = false

		k8sClient, err = k8sclient.New(c)
		if err != nil {
			t.Fatal(err)
		}
	}

	return k8sClient
}
