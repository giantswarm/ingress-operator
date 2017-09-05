package ingresstpr

import (
	"io/ioutil"
	"testing"

	"github.com/giantswarm/ingresstpr/spec"
	"github.com/giantswarm/ingresstpr/spec/hostcluster"
	"github.com/kylelemons/godebug/pretty"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestSpecYamlEncoding(t *testing.T) {
	spec := Spec{
		GuestCluster: spec.GuestCluster{
			ID:        "weof7",
			Namespace: "weof7",
			Service:   "worker",
		},
		HostCluster: spec.HostCluster{
			IngressController: hostcluster.IngressController{
				ConfigMap: "ingress-nginx-tcp-services",
				Namespace: "kube-system",
				Service:   "nginx-ingress-controller",
			},
		},
		ProtocolPorts: []spec.ProtocolPort{
			{
				IngressPort: 30010,
				LBPort:      30034,
				Protocol:    "http",
			},
			{
				IngressPort: 30011,
				LBPort:      30035,
				Protocol:    "https",
			},
		},
	}

	var got map[string]interface{}
	{
		bytes, err := yaml.Marshal(&spec)
		require.NoError(t, err, "marshaling spec")
		err = yaml.Unmarshal(bytes, &got)
		require.NoError(t, err, "unmarshaling spec to map")
	}

	var want map[string]interface{}
	{
		bytes, err := ioutil.ReadFile("testdata/spec.yaml")
		require.NoError(t, err)
		err = yaml.Unmarshal(bytes, &want)
		require.NoError(t, err, "unmarshaling fixture to map")
	}

	diff := pretty.Compare(want, got)
	require.Equal(t, "", diff, "diff: (-want +got)\n%s", diff)
}
