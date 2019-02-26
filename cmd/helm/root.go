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

package main // import "k8s.io/helm/cmd/helm"

import (
	"io"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/action"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/registry"
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
  $HELM_HOME          set an alternative location for Helm files. By default, these are stored in
                      "$XDG_CONFIG_DIR/helm" ("$HOME/.config/helm" on Linux,
                      "%APPDATA%\helm" on Windows and "$HOME/Library/Preferences" on OSX).
                      NOTE: if there is old-style "$HOME/.helm" directory, it will be used.
  $HELM_DRIVER        set the backend storage driver. Values are: configmap, secret, memory
  $HELM_NO_PLUGINS    disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.
  $KUBECONFIG         set an alternative Kubernetes configuration file (default "$HOME/.kube/config")
`

// TODO: 'c helm.Interface' is deprecated in favor of actionConfig
func newRootCmd(c helm.Interface, actionConfig *action.Configuration, out io.Writer, args []string) *cobra.Command {
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
		newCreateCmd(out),
		newDependencyCmd(out),
		newPullCmd(out),
		newShowCmd(out),
		newLintCmd(out),
		newPackageCmd(out),
		newRepoCmd(out),
		newSearchCmd(out),
		newVerifyCmd(out),
		newChartCmd(actionConfig, out),

		// release commands
		newGetCmd(c, out),
		newHistoryCmd(c, out),
		newInstallCmd(actionConfig, out),
		newListCmd(actionConfig, out),
		newReleaseTestCmd(c, out),
		newRollbackCmd(c, out),
		newStatusCmd(c, out),
		newUninstallCmd(c, out),
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

	// Find and add plugins
	loadPlugins(cmd, out)

	return cmd
}
