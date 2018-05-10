/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main // import "k8s.io/helm/cmd/helm"

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/storage/driver"
)

var (
	settings   environment.EnvSettings
	config     clientcmd.ClientConfig
	configOnce sync.Once
)

var globalUsage = `The Kubernetes package manager

To begin working with Helm, run the 'helm init' command:

	$ helm init

This will set up any necessary local configuration.

Common actions from this point include:

- helm search:    search for charts
- helm fetch:     download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment:
  $HELM_HOME          set an alternative location for Helm files. By default, these are stored in ~/.helm
  $HELM_NO_PLUGINS    disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.
  $KUBECONFIG         set an alternative Kubernetes configuration file (default "~/.kube/config")
`

func newRootCmd(c helm.Interface, out io.Writer, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		Long:         globalUsage,
		SilenceUsage: true,
	}
	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)

	cmd.AddCommand(
		// chart commands
		newCreateCmd(out),
		newDependencyCmd(out),
		newFetchCmd(out),
		newInspectCmd(out),
		newLintCmd(out),
		newPackageCmd(out),
		newRepoCmd(out),
		newSearchCmd(out),
		newVerifyCmd(out),

		// release commands
		newDeleteCmd(c, out),
		newGetCmd(c, out),
		newHistoryCmd(c, out),
		newInstallCmd(c, out),
		newListCmd(c, out),
		newReleaseTestCmd(c, out),
		newRollbackCmd(c, out),
		newStatusCmd(c, out),
		newUpgradeCmd(c, out),

		newCompletionCmd(out),
		newHomeCmd(out),
		newInitCmd(out),
		newPluginCmd(out),
		newTemplateCmd(out),
		newVersionCmd(out),

		// Hidden documentation generator command: 'helm docs'
		newDocsCmd(out),
	)

	flags.Parse(args)

	// set defaults from environment
	settings.Init(flags)

	// Find and add plugins
	loadPlugins(cmd, out)

	return cmd
}

func init() {
	log.SetFlags(log.Lshortfile)
}

func logf(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func main() {
	cmd := newRootCmd(nil, os.Stdout, os.Args[1:])
	if err := cmd.Execute(); err != nil {
		logf("%+v", err)
		os.Exit(1)
	}
}

func checkArgsLength(argsReceived int, requiredArgs ...string) error {
	expectedNum := len(requiredArgs)
	if argsReceived != expectedNum {
		arg := "arguments"
		if expectedNum == 1 {
			arg = "argument"
		}
		return errors.Errorf("this command needs %v %s: %s", expectedNum, arg, strings.Join(requiredArgs, ", "))
	}
	return nil
}

// ensureHelmClient returns a new helm client impl. if h is not nil.
func ensureHelmClient(h helm.Interface, allNamespaces bool) helm.Interface {
	if h != nil {
		return h
	}
	return newClient(allNamespaces)
}

func newClient(allNamespaces bool) helm.Interface {
	kc := kube.New(kubeConfig())
	kc.Log = logf

	clientset, err := kc.KubernetesClientSet()
	if err != nil {
		// TODO return error
		log.Fatal(err)
	}
	var namespace string
	if !allNamespaces {
		namespace = getNamespace()
	}
	// TODO add other backends
	d := driver.NewSecrets(clientset.CoreV1().Secrets(namespace))
	d.Log = logf

	return helm.NewClient(
		helm.KubeClient(kc),
		helm.Driver(d),
		helm.Discovery(clientset.Discovery()),
	)
}

func kubeConfig() clientcmd.ClientConfig {
	configOnce.Do(func() {
		config = kube.GetConfig(settings.KubeConfig, settings.KubeContext, settings.Namespace)
	})
	return config
}

func getNamespace() string {
	if ns, _, err := kubeConfig().Namespace(); err == nil {
		return ns
	}
	return "default"
}
