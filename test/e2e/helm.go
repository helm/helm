// build +e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
)

const (
	namespace = "dm"
	apiProxy  = "/api/v1/proxy/namespaces/" + namespace + "/services/manager-service:manager/"
)

type HelmContext struct {
	t    *testing.T
	Path string
	Host string
}

func NewHelmContext(t *testing.T) *HelmContext {
	return &HelmContext{
		t:    t,
		Path: RepoRoot() + "/bin/helm",
	}
}

func (h *HelmContext) Run(args ...string) *HelmCmd {
	cmd := h.newCmd()
	if status := cmd.exec(args...); status != nil {
		h.t.Fatalf("helm %v failed unexpectedly: %v", args, status)
	}
	return cmd
}

func (h *HelmContext) newCmd() *HelmCmd {
	return &HelmCmd{
		ctx: h,
	}
}

type HelmCmd struct {
	ctx            *HelmContext
	path           string
	ran            bool
	stdout, stderr bytes.Buffer
}

func (h *HelmCmd) exec(args ...string) error {
	args = append([]string{"--host", h.ctx.Host}, args...)
	cmd := exec.Command(h.ctx.Path, args...)
	h.stdout.Reset()
	h.stderr.Reset()
	cmd.Stdout = &h.stdout
	cmd.Stderr = &h.stderr
	status := cmd.Run()
	if h.stdout.Len() > 0 {
		h.ctx.t.Log("standard output:")
		h.ctx.t.Log(h.stdout.String())
	}
	if h.stderr.Len() > 0 {
		h.ctx.t.Log("standard error:")
		h.ctx.t.Log(h.stderr.String())
	}
	h.ran = true
	return status
}

// Stdout returns standard output of the helmCmd run as a string.
func (h *HelmCmd) Stdout() string {
	if !h.ran {
		h.ctx.t.Fatal("internal testsuite error: stdout called before run")
	}
	return h.stdout.String()
}

// Stderr returns standard error of the helmCmd run as a string.
func (h *HelmCmd) Stderr() string {
	if !h.ran {
		h.ctx.t.Fatal("internal testsuite error: stdout called before run")
	}
	return h.stderr.String()
}

func RepoRoot() string {
	return filepath.Clean(filepath.Join(path.Base(os.Args[0]), "../../.."))
}
