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
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
)

var globalUsage = `The Kubernetes package manager

Common actions for Helm:

- helm search:    search for charts
- helm pull:      download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment variables:

| Name                               | Description                                                                       |
|------------------------------------|-----------------------------------------------------------------------------------|
| $HELM_CACHE_HOME                   | set an alternative location for storing cached files.                             |
| $HELM_CONFIG_HOME                  | set an alternative location for storing Helm configuration.                       |
| $HELM_DATA_HOME                    | set an alternative location for storing Helm data.                                |
| $HELM_DEBUG                        | indicate whether or not Helm is running in Debug mode                             |
| $HELM_DRIVER                       | set the backend storage driver. Values are: configmap, secret, memory, postgres   |
| $HELM_DRIVER_SQL_CONNECTION_STRING | set the connection string the SQL storage driver should use.                      |
| $HELM_MAX_HISTORY                  | set the maximum number of helm release history.                                   |
| $HELM_NAMESPACE                    | set the namespace used for the helm operations.                                   |
| $HELM_NO_PLUGINS                   | disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.                        |
| $HELM_PLUGINS                      | set the path to the plugins directory                                             |
| $HELM_REGISTRY_CONFIG              | set the path to the registry config file.                                         |
| $HELM_REPOSITORY_CACHE             | set the path to the repository cache directory                                    |
| $HELM_REPOSITORY_CONFIG            | set the path to the repositories file.                                            |
| $KUBECONFIG                        | set an alternative Kubernetes configuration file (default "~/.kube/config")       |
| $HELM_KUBEAPISERVER                | set the Kubernetes API Server Endpoint for authentication                         |
| $HELM_KUBECAFILE                   | set the Kubernetes certificate authority file.                                    |
| $HELM_KUBEASGROUPS                 | set the Groups to use for impersonation using a comma-separated list.             |
| $HELM_KUBEASUSER                   | set the Username to impersonate for the operation.                                |
| $HELM_KUBECONTEXT                  | set the name of the kubeconfig context.                                           |
| $HELM_KUBETOKEN                    | set the Bearer KubeToken used for authentication.                                 |

Helm stores cache, configuration, and data based on the following configuration order:

- If a HELM_*_HOME environment variable is set, it will be used
- Otherwise, on systems supporting the XDG base directory specification, the XDG variables will be used
- When no other location is set a default location will be used based on the operating system

By default, the default directories depend on the Operating System. The defaults are listed below:

| Operating System | Cache Path                | Configuration Path             | Data Path               |
|------------------|---------------------------|--------------------------------|-------------------------|
| Linux            | $HOME/.cache/helm         | $HOME/.config/helm             | $HOME/.local/share/helm |
| macOS            | $HOME/Library/Caches/helm | $HOME/Library/Preferences/helm | $HOME/Library/helm      |
| Windows          | %TEMP%\helm               | %APPDATA%\helm                 | %APPDATA%\helm          |
`

func newRootCmd(actionConfig *action.Configuration, out io.Writer, args []string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		Long:         globalUsage,
		SilenceUsage: true,
	}
	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)
	addKlogFlags(flags)

	// Setup shell completion for the namespace flag
	err := cmd.RegisterFlagCompletionFunc("namespace", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if client, err := actionConfig.KubernetesClientSet(); err == nil {
			// Choose a long enough timeout that the user notices something is not working
			// but short enough that the user is not made to wait very long
			to := int64(3)
			cobra.CompDebugln(fmt.Sprintf("About to call kube client for namespaces with timeout of: %d", to), settings.Debug)

			nsNames := []string{}
			if namespaces, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{TimeoutSeconds: &to}); err == nil {
				for _, ns := range namespaces.Items {
					if strings.HasPrefix(ns.Name, toComplete) {
						nsNames = append(nsNames, ns.Name)
					}
				}
				return nsNames, cobra.ShellCompDirectiveNoFileComp
			}
		}
		return nil, cobra.ShellCompDirectiveDefault
	})

	if err != nil {
		log.Fatal(err)
	}

	// Setup shell completion for the kube-context flag
	err = cmd.RegisterFlagCompletionFunc("kube-context", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		cobra.CompDebugln("About to get the different kube-contexts", settings.Debug)

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if len(settings.KubeConfig) > 0 {
			loadingRules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: settings.KubeConfig}
		}
		if config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			&clientcmd.ConfigOverrides{}).RawConfig(); err == nil {
			comps := []string{}
			for name, context := range config.Contexts {
				if strings.HasPrefix(name, toComplete) {
					comps = append(comps, fmt.Sprintf("%s\t%s", name, context.Cluster))
				}
			}
			return comps, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	})

	if err != nil {
		log.Fatal(err)
	}

	// We can safely ignore any errors that flags.Parse encounters since
	// those errors will be caught later during the call to cmd.Execution.
	// This call is required to gather configuration information prior to
	// execution.
	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.Parse(args)

	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptWriter(out),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, err
	}
	actionConfig.RegistryClient = registryClient

	// Add subcommands
	cmd.AddCommand(
		// chart commands
		newCreateCmd(out),
		newDependencyCmd(actionConfig, out),
		newPullCmd(actionConfig, out),
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
	)

	// Add *experimental* subcommands
	cmd.AddCommand(
		newRegistryCmd(actionConfig, out),
		newChartCmd(actionConfig, out),
	)

	// Find and add plugins
	loadPlugins(cmd, out)

	// Check permissions on critical files
	checkPerms()

	// Check for expired repositories
	checkForExpiredRepos(settings.RepositoryConfig)

	return cmd, nil
}

func checkForExpiredRepos(repofile string) {

	expiredRepos := []struct {
		name string
		old  string
		new  string
	}{
		{
			name: "stable",
			old:  "kubernetes-charts.storage.googleapis.com",
			new:  "https://charts.helm.sh/stable",
		},
		{
			name: "incubator",
			old:  "kubernetes-charts-incubator.storage.googleapis.com",
			new:  "https://charts.helm.sh/incubator",
		},
	}

	// parse repo file.
	// Ignore the error because it is okay for a repo file to be unparseable at this
	// stage. Later checks will trap the error and respond accordingly.
	repoFile, err := repo.LoadFile(repofile)
	if err != nil {
		return
	}

	for _, exp := range expiredRepos {
		r := repoFile.Get(exp.name)
		if r == nil {
			return
		}

		if url := r.URL; strings.Contains(url, exp.old) {
			fmt.Fprintf(
				os.Stderr,
				"WARNING: %q is deprecated for %q and will be deleted Nov. 13, 2020.\nWARNING: You should switch to %q via:\nWARNING: helm repo add %q %q --force-update\n",
				exp.old,
				exp.name,
				exp.new,
				exp.name,
				exp.new,
			)
		}
	}

}
