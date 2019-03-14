/*
Copyright The Helm Authors.
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

package cli

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // initialize kubernetes client auth plugins

	"k8s.io/helm/pkg/action"
	"k8s.io/helm/pkg/cli/environment"
	"k8s.io/helm/pkg/cli/require"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/registry"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
)

const globalUsage = `The Kubernetes package manager

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
  $HELM_DRIVER        set the backend storage driver. Values are: configmap, secret, memory
  $HELM_NO_PLUGINS    disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.
  $KUBECONFIG         set an alternative Kubernetes configuration file (default "~/.kube/config")
`

var (
	settings   environment.Settings
	config     genericclioptions.RESTClientGetter
	configOnce sync.Once
)

func New(actionConfig *action.Configuration, out io.Writer, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		Long:         globalUsage,
		SilenceUsage: true,
		Args:         require.NoArgs,
	}
	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)

	flags.Parse(args)

	// set defaults from environment
	settings.Init(flags)

	// Add the registry client based on settings
	// TODO: Move this elsewhere (first, settings.Init() must move)
	actionConfig.RegistryClient = registry.NewClient(&registry.ClientOptions{
		Out: out,
		Resolver: registry.Resolver{
			Resolver: docker.NewResolver(docker.ResolverOptions{}),
		},
		CacheRootDir: settings.Home.Registry(),
	})

	cmd.AddCommand(
		// chart commands
		NewCreateCmd(out),
		NewDependencyCmd(out),
		NewPullCmd(out),
		NewShowCmd(out),
		NewLintCmd(out),
		NewPackageCmd(out),
		NewRepoCmd(out),
		NewSearchCmd(out),
		NewVerifyCmd(out),
		NewChartCmd(actionConfig, out),

		// release commands
		NewGetCmd(actionConfig, out),
		NewHistoryCmd(actionConfig, out),
		NewInstallCmd(actionConfig, out),
		NewListCmd(actionConfig, out),
		NewReleaseTestCmd(actionConfig, out),
		NewRollbackCmd(actionConfig, out),
		NewStatusCmd(actionConfig, out),
		NewUninstallCmd(actionConfig, out),
		NewUpgradeCmd(actionConfig, out),

		NewCompletionCmd(out),
		NewHomeCmd(out),
		NewInitCmd(out),
		NewPluginCmd(out),
		NewTemplateCmd(out),
		NewVersionCmd(out),

		// Hidden documentation generator command: 'helm docs'
		NewDocsCmd(out),
	)

	// Find and add plugins
	loadPlugins(cmd, out)

	return cmd
}

func logf(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func NewActionConfig(allNamespaces bool) *action.Configuration {
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

	var store *storage.Storage
	switch os.Getenv("HELM_DRIVER") {
	case "secret", "secrets", "":
		d := driver.NewSecrets(clientset.CoreV1().Secrets(namespace))
		d.Log = logf
		store = storage.Init(d)
	case "configmap", "configmaps":
		d := driver.NewConfigMaps(clientset.CoreV1().ConfigMaps(namespace))
		d.Log = logf
		store = storage.Init(d)
	case "memory":
		d := driver.NewMemory()
		store = storage.Init(d)
	default:
		// Not sure what to do here.
		panic("Unknown driver in HELM_DRIVER: " + os.Getenv("HELM_DRIVER"))
	}

	return &action.Configuration{
		KubeClient: kc,
		Releases:   store,
		Discovery:  clientset.Discovery(),
	}
}

func kubeConfig() genericclioptions.RESTClientGetter {
	configOnce.Do(func() {
		config = kube.GetConfig(settings.KubeConfig, settings.KubeContext, settings.Namespace)
	})
	return config
}

func getNamespace() string {
	if ns, _, err := kubeConfig().ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return "default"
}
