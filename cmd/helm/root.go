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

package main // import "helm.sh/helm/v3/cmd/helm"

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/action"
)

var globalUsage = `The Kubernetes package manager

Common actions for Helm:

- helm search:    search for charts
- helm pull:      download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment variables:

+------------------+--------------------------------------------------------------------------------------------------------+
| Name                                  | Description                                                                       |
+------------------+--------------------------------------------------------------------------------------------------------+
| $XDG_CACHE_HOME                       | set an alternative location for storing cached files.                             |
| $XDG_CONFIG_HOME                      | set an alternative location for storing Helm configuration.                       |
| $XDG_DATA_HOME                        | set an alternative location for storing Helm data.                                |
| $HELM_DRIVER                          | set the backend storage driver. Values are: configmap, secret, memory, postgres   |
| $HELM_DRIVER_SQL_CONNECTION_STRING    | set the connection string the SQL storage driver should use.                      |
| $HELM_NO_PLUGINS                      | disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.                        |
| $KUBECONFIG                           | set an alternative Kubernetes configuration file (default "~/.kube/config")       |
+------------------+--------------------------------------------------------------------------------------------------------+

Helm stores configuration based on the XDG base directory specification, so

- cached files are stored in $XDG_CACHE_HOME/helm
- configuration is stored in $XDG_CONFIG_HOME/helm
- data is stored in $XDG_DATA_HOME/helm

By default, the default directories depend on the Operating System. The defaults are listed below:

+------------------+---------------------------+--------------------------------+-------------------------+
| Operating System | Cache Path                | Configuration Path             | Data Path               |
+------------------+---------------------------+--------------------------------+-------------------------+
| Linux            | $HOME/.cache/helm         | $HOME/.config/helm             | $HOME/.local/share/helm |
| macOS            | $HOME/Library/Caches/helm | $HOME/Library/Preferences/helm | $HOME/Library/helm      |
| Windows          | %TEMP%\helm               | %APPDATA%\helm                 | %APPDATA%\helm          |
+------------------+---------------------------+--------------------------------+-------------------------+
`

func newRootCmd(actionConfig *action.Configuration, out io.Writer, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                    "helm",
		Short:                  "The Helm package manager for Kubernetes.",
		Long:                   globalUsage,
		SilenceUsage:           true,
		BashCompletionFunction: completion.GetBashCustomFunction(),
	}
	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)

	// Setup shell completion for the namespace flag
	flag := flags.Lookup("namespace")
	completion.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if client, err := actionConfig.KubernetesClientSet(); err == nil {
			// Choose a long enough timeout that the user notices somethings is not working
			// but short enough that the user is not made to wait very long
			to := int64(3)
			completion.CompDebugln(fmt.Sprintf("About to call kube client for namespaces with timeout of: %d", to))

			nsNames := []string{}
			if namespaces, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{TimeoutSeconds: &to}); err == nil {
				for _, ns := range namespaces.Items {
					if strings.HasPrefix(ns.Name, toComplete) {
						nsNames = append(nsNames, ns.Name)
					}
				}
				return nsNames, completion.BashCompDirectiveNoFileComp
			}
		}
		return nil, completion.BashCompDirectiveDefault
	})

	// Setup shell completion for the kube-context flag
	flag = flags.Lookup("kube-context")
	completion.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		completion.CompDebugln("About to get the different kube-contexts")

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if len(settings.KubeConfig) > 0 {
			loadingRules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: settings.KubeConfig}
		}
		if config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			&clientcmd.ConfigOverrides{}).RawConfig(); err == nil {
			ctxs := []string{}
			for name := range config.Contexts {
				if strings.HasPrefix(name, toComplete) {
					ctxs = append(ctxs, name)
				}
			}
			return ctxs, completion.BashCompDirectiveNoFileComp
		}
		return nil, completion.BashCompDirectiveNoFileComp
	})

	// We can safely ignore any errors that flags.Parse encounters since
	// those errors will be caught later during the call to cmd.Execution.
	// This call is required to gather configuration information prior to
	// execution.
	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.Parse(args)

	// Add subcommands
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

		// release commands
		newGetCmd(actionConfig, out),
		newHistoryCmd(actionConfig, out),
		newInstallCmd(actionConfig, out),
		newListCmd(actionConfig, out),
		newReleaseTestCmd(actionConfig, out),
		newRollbackCmd(actionConfig, out),
		newStatusCmd(actionConfig, out),
		newTemplateCmd(actionConfig, out),
		newUninstallCmd(actionConfig, out),
		newUpgradeCmd(actionConfig, out),

		newCompletionCmd(out),
		newEnvCmd(out),
		newPluginCmd(out),
		newVersionCmd(out),

		// Hidden documentation generator command: 'helm docs'
		newDocsCmd(out),

		// Setup the special hidden __complete command to allow for dynamic auto-completion
		completion.NewCompleteCmd(settings, out),
	)

	// Add *experimental* subcommands
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptWriter(out),
	)
	if err != nil {
		// TODO: don't panic here, refactor newRootCmd to return error
		panic(err)
	}
	actionConfig.RegistryClient = registryClient
	cmd.AddCommand(
		newRegistryCmd(actionConfig, out),
		newChartCmd(actionConfig, out),
	)

	// Find and add plugins
	loadPlugins(cmd, out)

	return cmd
}
