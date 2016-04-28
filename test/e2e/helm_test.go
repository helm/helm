// build +e2e

package e2e

import (
	"flag"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

const (
	timeout = 180 * time.Second
	poll    = 2 * time.Second
)

var (
	chart       = flag.String("chart", "gs://kubernetes-charts-testing/redis-2.tgz", "Chart to deploy")
	tillerImage = flag.String("tiller-image", "", "The full image name of the Docker image for resourcifier.")
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

	if !isHelmRunning(kube) {
		args := []string{"init"}
		if *tillerImage != "" {
			args = append(args, "-i", *tillerImage)
		}

		helm.MustRun(args...)

		err := wait(func() bool {
			return isHelmRunning(kube)
		})

		if err != nil {
			t.Fatalf("could not install helm: %s", err)
		}
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

func isHelmRunning(k *KubeContext) bool {
	return k.Run("get", "pods", "--namespace=helm").StdoutContains("Running")
}
