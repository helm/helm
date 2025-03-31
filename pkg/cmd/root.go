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

package cmd // import "helm.sh/helm/v4/pkg/cmd"

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	logadapter "helm.sh/helm/v4/internal/log"
	"helm.sh/helm/v4/internal/tlsutil"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	"helm.sh/helm/v4/pkg/registry"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/repo"
	"helm.sh/helm/v4/pkg/storage/driver"
)

var globalUsage = `The Kubernetes package manager

Common actions for Helm:

- helm search:    search for charts
- helm pull:      download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment variables:

| Name                               | Description                                                                                                |
|------------------------------------|------------------------------------------------------------------------------------------------------------|
| $HELM_CACHE_HOME                   | set an alternative location for storing cached files.                                                      |
| $HELM_CONFIG_HOME                  | set an alternative location for storing Helm configuration.                                                |
| $HELM_DATA_HOME                    | set an alternative location for storing Helm data.                                                         |
| $HELM_DEBUG                        | indicate whether or not Helm is running in Debug mode                                                      |
| $HELM_DRIVER                       | set the backend storage driver. Values are: configmap, secret, memory, sql.                                |
| $HELM_DRIVER_SQL_CONNECTION_STRING | set the connection string the SQL storage driver should use.                                               |
| $HELM_MAX_HISTORY                  | set the maximum number of helm release history.                                                            |
| $HELM_NAMESPACE                    | set the namespace used for the helm operations.                                                            |
| $HELM_NO_PLUGINS                   | disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.                                                 |
| $HELM_PLUGINS                      | set the path to the plugins directory                                                                      |
| $HELM_REGISTRY_CONFIG              | set the path to the registry config file.                                                                  |
| $HELM_REPOSITORY_CACHE             | set the path to the repository cache directory                                                             |
| $HELM_REPOSITORY_CONFIG            | set the path to the repositories file.                                                                     |
| $KUBECONFIG                        | set an alternative Kubernetes configuration file (default "~/.kube/config")                                |
| $HELM_KUBEAPISERVER                | set the Kubernetes API Server Endpoint for authentication                                                  |
| $HELM_KUBECAFILE                   | set the Kubernetes certificate authority file.                                                             |
| $HELM_KUBEASGROUPS                 | set the Groups to use for impersonation using a comma-separated list.                                      |
| $HELM_KUBEASUSER                   | set the Username to impersonate for the operation.                                                         |
| $HELM_KUBECONTEXT                  | set the name of the kubeconfig context.                                                                    |
| $HELM_KUBETOKEN                    | set the Bearer KubeToken used for authentication.                                                          |
| $HELM_KUBEINSECURE_SKIP_TLS_VERIFY | indicate if the Kubernetes API server's certificate validation should be skipped (insecure)                |
| $HELM_KUBETLS_SERVER_NAME          | set the server name used to validate the Kubernetes API server certificate                                 |
| $HELM_BURST_LIMIT                  | set the default burst limit in the case the server contains many CRDs (default 100, -1 to disable)         |
| $HELM_QPS                          | set the Queries Per Second in cases where a high number of calls exceed the option for higher burst values |

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

var settings = cli.New()
var logger = logadapter.NewReadableTextLogger(os.Stderr, settings.Debug)

func NewRootCmd(out io.Writer, args []string) (*cobra.Command, error) {
	actionConfig := new(action.Configuration)
	cmd, err := newRootCmdWithConfig(actionConfig, out, args)
	if err != nil {
		return nil, err
	}
	cobra.OnInitialize(func() {
		helmDriver := os.Getenv("HELM_DRIVER")
		if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, logger); err != nil {
			log.Fatal(err)
		}
		if helmDriver == "memory" {
			loadReleasesInMemory(actionConfig)
		}
		actionConfig.SetHookOutputFunc(hookOutputWriter)
	})
	return cmd, nil
}

func newRootCmdWithConfig(actionConfig *action.Configuration, out io.Writer, args []string) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		Long:         globalUsage,
		SilenceUsage: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if err := startProfiling(); err != nil {
				log.Printf("Warning: Failed to start profiling: %v", err)
			}
		},
		PersistentPostRun: func(_ *cobra.Command, _ []string) {
			if err := stopProfiling(); err != nil {
				log.Printf("Warning: Failed to stop profiling: %v", err)
			}
		},
	}

	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)
	addKlogFlags(flags)

	// Setup shell completion for the namespace flag
	err := cmd.RegisterFlagCompletionFunc("namespace", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		if client, err := actionConfig.KubernetesClientSet(); err == nil {
			// Choose a long enough timeout that the user notices something is not working
			// but short enough that the user is not made to wait very long
			to := int64(3)
			cobra.CompDebugln(fmt.Sprintf("About to call kube client for namespaces with timeout of: %d", to), settings.Debug)

			nsNames := []string{}
			if namespaces, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{TimeoutSeconds: &to}); err == nil {
				for _, ns := range namespaces.Items {
					nsNames = append(nsNames, ns.Name)
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
	err = cmd.RegisterFlagCompletionFunc("kube-context", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
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
				comps = append(comps, fmt.Sprintf("%s\t%s", name, context.Cluster))
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

	registryClient, err := newDefaultRegistryClient(false, "", "")
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
		newShowCmd(actionConfig, out),
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

	cmd.AddCommand(
		newRegistryCmd(actionConfig, out),
		newPushCmd(actionConfig, out),
	)

	// Find and add plugins
	loadPlugins(cmd, out)

	// Check for expired repositories
	checkForExpiredRepos(settings.RepositoryConfig)

	return cmd, nil
}

// This function loads releases into the memory storage if the
// environment variable is properly set.
func loadReleasesInMemory(actionConfig *action.Configuration) {
	filePaths := strings.Split(os.Getenv("HELM_MEMORY_DRIVER_DATA"), ":")
	if len(filePaths) == 0 {
		return
	}

	store := actionConfig.Releases
	mem, ok := store.Driver.(*driver.Memory)
	if !ok {
		// For an unexpected reason we are not dealing with the memory storage driver.
		return
	}

	actionConfig.KubeClient = &kubefake.PrintingKubeClient{Out: io.Discard}

	for _, path := range filePaths {
		b, err := os.ReadFile(path)
		if err != nil {
			log.Fatal("Unable to read memory driver data", err)
		}

		releases := []*release.Release{}
		if err := yaml.Unmarshal(b, &releases); err != nil {
			log.Fatal("Unable to unmarshal memory driver data: ", err)
		}

		for _, rel := range releases {
			if err := store.Create(rel); err != nil {
				log.Fatal(err)
			}
		}
	}
	// Must reset namespace to the proper one
	mem.SetNamespace(settings.Namespace())
}

// hookOutputWriter provides the writer for writing hook logs.
func hookOutputWriter(_, _, _ string) io.Writer {
	return log.Writer()
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
	// Ignore the error because it is okay for a repo file to be unparsable at this
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

func newRegistryClient(
	certFile, keyFile, caFile string, insecureSkipTLSverify, plainHTTP bool, username, password string,
) (*registry.Client, error) {
	if certFile != "" && keyFile != "" || caFile != "" || insecureSkipTLSverify {
		registryClient, err := newRegistryClientWithTLS(certFile, keyFile, caFile, insecureSkipTLSverify, username, password)
		if err != nil {
			return nil, err
		}
		return registryClient, nil
	}
	registryClient, err := newDefaultRegistryClient(plainHTTP, username, password)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}

func newDefaultRegistryClient(plainHTTP bool, username, password string) (*registry.Client, error) {
	opts := []registry.ClientOption{
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(os.Stderr),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
		registry.ClientOptBasicAuth(username, password),
	}
	if plainHTTP {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}

	// Create a new registry client
	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}

func newRegistryClientWithTLS(
	certFile, keyFile, caFile string, insecureSkipTLSverify bool, username, password string,
) (*registry.Client, error) {
	tlsConf, err := tlsutil.NewTLSConfig(
		tlsutil.WithInsecureSkipVerify(insecureSkipTLSverify),
		tlsutil.WithCertKeyPairFiles(certFile, keyFile),
		tlsutil.WithCAFile(caFile),
	)

	if err != nil {
		return nil, fmt.Errorf("can't create TLS config for client: %w", err)
	}

	// Create a new registry client
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(os.Stderr),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
		registry.ClientOptHTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConf,
				Proxy:           http.ProxyFromEnvironment,
			},
		}),
		registry.ClientOptBasicAuth(username, password),
	)
	if err != nil {
		return nil, err
	}
	return registryClient, nil
}
