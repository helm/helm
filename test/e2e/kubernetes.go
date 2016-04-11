// build +e2e

package e2e

import (
	"strings"
	"testing"
)

const defaultKubectlPath = "kubectl"

type KubeContext struct {
	t    *testing.T
	Path string
}

func NewKubeContext(t *testing.T) *KubeContext {
	return &KubeContext{
		t:    t,
		Path: defaultKubectlPath,
	}
}

func (k *KubeContext) Run(args ...string) *Cmd {
	cmd := k.newCmd(args...)
	cmd.exec()
	return cmd
}

func (k *KubeContext) newCmd(args ...string) *Cmd {
	return &Cmd{
		t:    k.t,
		path: k.Path,
		args: args,
	}
}

func (k *KubeContext) getConfigValue(jsonpath string) string {
	return strings.Replace(k.Run("config", "view", "--flatten=true", "--minify=true", "-o", "jsonpath="+jsonpath).Stdout(), "'", "", -1)
}

func (k *KubeContext) Cluster() string {
	return k.getConfigValue("'{.clusters[0].name}'")
}

func (k *KubeContext) Server() string {
	return k.getConfigValue("'{.clusters[0].cluster.server}'")
}

func (k *KubeContext) CurrentContext() string {
	return k.getConfigValue("'{.current-context}'")
}

func (k *KubeContext) Running() bool {
	err := k.Run("cluster-info").exec()
	return err == nil
}

func (k *KubeContext) Version() string {
	return k.Run("version").Stdout()
}
