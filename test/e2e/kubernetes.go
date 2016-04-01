// build +e2e

package e2e

import (
	"os/exec"
	"strings"
)

const defaultKubectlPath = "kubectl"

type KubeContext struct {
	Path string
}

func NewKubeContext() *KubeContext {
	return &KubeContext{
		Path: defaultKubectlPath,
	}
}

func (k *KubeContext) Cluster() string {
	out, _ := exec.Command(k.Path, "config", "view", "--flatten=true", "--minify=true", "-o", "jsonpath='{.clusters[0].name}'").Output()
	return string(out)
}

func (k *KubeContext) Server() string {
	out, _ := exec.Command(k.Path, "config", "view", "--flatten=true", "--minify=true", "-o", "jsonpath='{.clusters[0].cluster.server}'").Output()
	return strings.Replace(string(out), "'", "", -1)
}

func (k *KubeContext) CurrentContext() string {
	out, _ := exec.Command(k.Path, "config", "view", "--flatten=true", "--minify=true", "-o", "jsonpath='{.current-context}'").Output()
	return string(out)
}

func (k *KubeContext) Running() bool {
	_, err := exec.Command(k.Path, "cluster-info").CombinedOutput()
	return err == nil
}

func (k *KubeContext) Version() string {
	out, _ := exec.Command(k.Path, "version").Output()
	return string(out)
}
