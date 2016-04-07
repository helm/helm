// build +e2e

package e2e

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

const (
	timeout = 10 * time.Second
	poll    = 2 * time.Second
)

var (
	repoURL  = flag.String("repo-url", "gs://kubernetes-charts-testing", "Repository URL")
	repoName = flag.String("repo-name", "kubernetes-charts-testing", "Repository name")
	chart    = flag.String("chart", "gs://kubernetes-charts-testing/redis-2.tgz", "Chart to deploy")
	host     = flag.String("host", "", "The URL to the helm server")

	resourcifierImage = "quay.io/adamreese/resourcifier:latest"
	expandybirdImage  = "quay.io/adamreese/expandybird:latest"
	managerImage      = "quay.io/adamreese/manager:latest"
)

func logKubeEnv(k *KubeContext) {
	config := k.Run("config", "view", "--flatten", "--minify").Stdout()
	k.t.Logf("Kubernetes Environment\n%s", config)
}

func TestHelm(t *testing.T) {
	kube := NewKubeContext(t)
	helm := NewHelmContext(t)

	logKubeEnv(kube)

	if !kube.Running() {
		t.Fatal("Not connected to kubernetes")
	}
	t.Log(kube.Version())
	t.Log(helm.MustRun("version").Stdout())

	helm.Host = helmHost()
	if helm.Host == "" {
		helm.Host = fmt.Sprintf("%s%s", kube.Server(), apiProxy)
	}
	t.Logf("Using host: %v", helm.Host)

	//TODO: skip check if running local binaries
	if !helm.Running() {
		t.Error("Helm is not installed")
		helm.MustRun(
			"server",
			"install",
			"--resourcifier-image", resourcifierImage,
			"--expandybird-image", expandybirdImage,
			"--manager-image", managerImage,
		)
		if err := wait(helm.Running); err != nil {
			t.Fatal(err)
		}
	}

	// Add repo if it does not exsit
	if !helm.MustRun("repo", "list").Contains(*repoURL) {
		t.Logf("Adding repo %s %s", *repoName, *repoURL)
		helm.MustRun("repo", "add", *repoName, *repoURL)
	}

	// Generate a name
	deploymentName := genName()

	t.Log("Executing deploy")
	helm.MustRun("deploy",
		"--properties", "namespace=e2e",
		"--name", deploymentName,
		*chart,
	)

	if err := wait(func() bool {
		return kube.Run("get", "pods").Match("redis.*Running")
	}); err != nil {
		t.Fatal(err)
	}
	t.Log(kube.Run("get", "pods").Stdout())

	t.Log("Executing deployment list")
	if !helm.MustRun("deployment", "list").Contains(deploymentName) {
		t.Fatal("Could not list deployment")
	}

	t.Log("Executing deployment info")
	if !helm.MustRun("deployment", "info", deploymentName).Contains("Deployed") {
		t.Fatal("Could not deploy")
	}

	t.Log("Executing deployment describe")
	helm.MustRun("deployment", "describe", deploymentName)

	t.Log("Executing deployment delete")
	if !helm.MustRun("deployment", "rm", deploymentName).Contains("Deleted") {
		t.Fatal("Could not delete deployment")
	}
}

type conditionFunc func() bool

func wait(fn conditionFunc) error {
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		if fn() {
			return nil
		}
	}
	return fmt.Errorf("Polling timeout")
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
