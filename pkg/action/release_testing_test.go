package action

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/release"
	clientset "k8s.io/client-go/kubernetes/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	"strings"
	"testing"
)

func TestGetPodLogs(t *testing.T) {
	is := assert.New(t)
	rel2 := releaseStub()
	rel2.Hooks[1].Kind = "Job"
	tests := []struct {
		name string
		rel  *release.Release
	}{
		{
			name: "output log from pod ",
			rel:  releaseStub(),
		},
	}
	for _, tt := range tests {

		buf := new(bytes.Buffer)
		cfg := actionConfigFixture(t)
		cfg.RESTClientGetter = cmdtesting.NewTestFactory().WithNamespace("default")
		rt := NewReleaseTesting(cfg)
		rt.clientSet = clientset.NewSimpleClientset()

		err := rt.GetPodLogs(buf, tt.rel)

		if !strings.Contains(buf.String(), "fake logs") {
			is.Error(err, "not get pod log for %s", "finding-nemo")
		}

		if err != nil {
			t.Error("should not return an error.")
		}
	}
}
