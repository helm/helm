// build +e2e

package e2e

import (
	"testing"
	"time"
)

const (
	namespace = "helm"
	apiProxy  = "/api/v1/proxy/namespaces/" + namespace + "/services/manager-service:manager/"
)

type HelmContext struct {
	t       *testing.T
	Path    string
	Host    string
	Timeout time.Duration
}

func NewHelmContext(t *testing.T) *HelmContext {
	return &HelmContext{
		t:       t,
		Path:    "../../bin/helm",
		Timeout: time.Second * 20,
	}
}

func (h *HelmContext) MustRun(args ...string) *Cmd {
	cmd := h.newCmd(args...)
	if status := cmd.exec(); status != nil {
		h.t.Errorf("helm %v failed unexpectedly: %v", args, status)
		h.t.Errorf("%s", cmd.Stderr())
		h.t.FailNow()
	}
	return cmd
}

func (h *HelmContext) Run(args ...string) *Cmd {
	cmd := h.newCmd(args...)
	cmd.exec()
	return cmd
}

func (h *HelmContext) RunFail(args ...string) *Cmd {
	cmd := h.newCmd(args...)
	if status := cmd.exec(); status == nil {
		h.t.Fatalf("helm unexpected to fail: %v %v", args, status)
	}
	return cmd
}

func (h *HelmContext) newCmd(args ...string) *Cmd {
	//args = append([]string{"--host", h.Host}, args...)
	return &Cmd{
		t:    h.t,
		path: h.Path,
		args: args,
	}
}

func (h *HelmContext) Running() bool {
	// FIXME tiller does not have a healthz endpoint
	return true

	//endpoint := h.Host + "healthz"
	//resp, err := http.Get(endpoint)
	//if err != nil {
	//h.t.Errorf("Could not GET %s: %s", endpoint, err)
	//}
	//return resp.StatusCode == 200
}
