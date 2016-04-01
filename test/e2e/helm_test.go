// build +e2e

package e2e

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

var (
	repoURL  = flag.String("repo-url", "gs://areese-charts", "Repository URL")
	repoName = flag.String("repo-name", "areese-charts", "Repository name")
	chart    = flag.String("chart", "gs://areese-charts/replicatedservice-3.tgz", "Chart to deploy")
	host     = flag.String("host", "", "The URL to the helm server")
)

func TestHelm(t *testing.T) {
	kube := NewKubeContext()
	helm := NewHelmContext(t)

	t.Log(kube.CurrentContext())
	t.Log(kube.Cluster())
	t.Log(kube.Server())

	if !kube.Running() {
		t.Fatal("Not connected to kubernetes")
	}

	t.Log("Kuberneter Version")
	t.Log(kube.Version())

	if !helmRunning(helm) {
		t.Fatal("Helm is not installed")
	}

	helm.Host = helmHost()

	if helm.Host == "" {
		helm.Host = fmt.Sprintf("%s%s", kube.Server(), apiProxy)
	}
	t.Logf("Using host: %v", helm.Host)

	if !helm.Run("repo", "list").Contains(*repoURL) {
		t.Logf("Adding repo %s %s", *repoName, *repoURL)
		helm.Run("repo", "add", *repoName, *repoURL)
	}

	deploymentName := genName()

	t.Log("Executing deploy")
	helm.Run("deploy", "--properties", "container_port=6379,image=kubernetes/redis:v1,replicas=2", "--name", deploymentName, *chart)

	t.Log("Executing deployment list")
	if !helm.Run("deployment", "list").Contains(deploymentName) {
		t.Fatal("Could not list deployment")
	}

	t.Log("Executing deployment info")
	if !helm.Run("deployment", "info", deploymentName).Contains("Deployed") {
		t.Fatal("Could not deploy")
	}

	t.Log("Executing deployment delete")
	if !helm.Run("deployment", "rm", deploymentName).Contains("Deleted") {
		t.Fatal("Could not delete deployment")
	}
}

func genName() string {
	return fmt.Sprintf("e2e-%d", rand.Uint32())
}

func helmHost() string {
	if *host != "" {
		return *host
	}
	return os.Getenv("HELM_HOST")
}

func helmRunning(h *HelmContext) bool {
	out := h.Run("server", "status").Stdout()
	return strings.Count(out, "Running") == 5
}
