package e2e

import (
	"os/exec"
	"strings"
	"testing"
)

func TestHelm(t *testing.T) {
	if !kubeRunning() {
		t.Fatal("Not connected to kubernetes")
	}

	helm := NewHelmContext(t)
	if !helmRunning(helm) {
		t.Fatal("Helm is not installed")
	}

	// Setup helm host

	// Run deploy

	// Test deployment
}

func helmRunning(h *HelmContext) bool {
	out := h.Run("server", "status").Stdout()
	return strings.Count(out, "Running") == 5
}

func kubeRunning() bool {
	_, err := exec.Command("kubectl", "cluster-info").CombinedOutput()
	return err == nil
}
