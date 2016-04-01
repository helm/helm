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

	resourcifierImage = "quay.io/adamreese/resourcifier:latest"
	expandybirdImage  = "quay.io/adamreese/expandybird:latest"
	managerImage      = "quay.io/adamreese/manager:latest"
)

func TestHelm(t *testing.T) {
	kube := NewKubeContext()
	helm := NewHelmContext(t)

	t.Logf("Kubenetes context: %s", kube.CurrentContext())
	t.Logf("Cluster: %s", kube.Cluster())
	t.Logf("Server: %s", kube.Server())

	if !kube.Running() {
		t.Fatal("Not connected to kubernetes")
	}

	t.Log(kube.Version())

	//TODO: skip check if running local binaries
	if !helmRunning(helm) {
		t.Error("Helm is not installed")
		helm.MustRun("server", "install", "--resourcifier-image", resourcifierImage, "--expandybird-image", expandybirdImage, "--manager-image", managerImage)
		//TODO: wait for pods to be ready
	}

	helm.Host = helmHost()

	if helm.Host == "" {
		helm.Host = fmt.Sprintf("%s%s", kube.Server(), apiProxy)
	}
	t.Logf("Using host: %v", helm.Host)

	// Add repo if it does not exsit
	if !helm.MustRun("repo", "list").Contains(*repoURL) {
		t.Logf("Adding repo %s %s", *repoName, *repoURL)
		helm.MustRun("repo", "add", *repoName, *repoURL)
	}

	// Generate a name
	deploymentName := genName()

	t.Log("Executing deploy")
	helm.MustRun("deploy", "--properties", "container_port=6379,image=kubernetes/redis:v1,replicas=2", "--name", deploymentName, *chart)

	t.Log("Executing deployment list")
	if !helm.MustRun("deployment", "list").Contains(deploymentName) {
		t.Fatal("Could not list deployment")
	}

	t.Log("Executing deployment info")
	if !helm.MustRun("deployment", "info", deploymentName).Contains("Deployed") {
		t.Fatal("Could not deploy")
	}

	t.Log("Executing deployment delete")
	if !helm.MustRun("deployment", "rm", deploymentName).Contains("Deleted") {
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
	out := h.MustRun("server", "status").Stdout()
	return strings.Count(out, "Running") == 5
}
