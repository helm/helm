// build +e2e

package e2e

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

type Cmd struct {
	t              *testing.T
	path           string
	args           []string
	ran            bool
	status         error
	stdout, stderr bytes.Buffer
}

func (h *Cmd) String() string {
	return fmt.Sprintf("%s %s", h.path, strings.Join(h.args, " "))
}

func (h *Cmd) exec() error {
	cmd := exec.Command(h.path, h.args...)
	h.stdout.Reset()
	h.stderr.Reset()
	cmd.Stdout = &h.stdout
	cmd.Stderr = &h.stderr

	h.t.Logf("Executing command: %s", h)
	h.status = cmd.Run()

	if h.stdout.Len() > 0 {
		h.t.Logf("standard output:\n%s", h.stdout.String())
	}
	if h.stderr.Len() > 0 {
		h.t.Logf("standard error: %s\n", h.stderr.String())
	}

	h.ran = true
	return h.status
}

// Stdout returns standard output of the Cmd run as a string.
func (h *Cmd) Stdout() string {
	if !h.ran {
		h.t.Fatal("internal testsuite error: stdout called before run")
	}
	return h.stdout.String()
}

// Stderr returns standard error of the Cmd run as a string.
func (h *Cmd) Stderr() string {
	if !h.ran {
		h.t.Fatal("internal testsuite error: stdout called before run")
	}
	return h.stderr.String()
}

func (c *Cmd) Match(exp string) bool {
	re := regexp.MustCompile(exp)
	return re.MatchString(c.Stdout())
}

func (h *Cmd) StdoutContains(substring string) bool {
	return strings.Contains(h.Stdout(), substring)
}

func (h *Cmd) StderrContains(substring string) bool {
	return strings.Contains(h.Stderr(), substring)
}

func (h *Cmd) Contains(substring string) bool {
	return h.StdoutContains(substring) || h.StderrContains(substring)
}
