package tests

import (
	"testing"

	"github.com/giantswarm/micrologger"
	"github.com/giantswarm/operatorkit/client/k8s"
	"k8s.io/client-go/kubernetes"
)

func K8sClient(t *testing.T) kubernetes.Interface {
	var err error

	var k8sClient kubernetes.Interface
	{
		var newLogger micrologger.Logger

		loggerConfig := micrologger.DefaultConfig()
		newLogger, err = micrologger.New(loggerConfig)
		if err != nil {
			t.Fatal(err)
		}

		config := k8s.Config{
			Logger:    newLogger,
			Address:   "http://127.0.0.1:8080",
			InCluster: false,
		}
		k8sClient, err = k8s.NewClient(config)
		if err != nil {
			t.Fatal(err)
		}
	}

	return k8sClient
}
