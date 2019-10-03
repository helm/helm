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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/portforwarder"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/tlsutil"
)

const (
	bashCompletionFunc = `
__helm_override_flag_list=(--kubeconfig --kube-context --host --tiller-namespace --home)
__helm_override_flags()
{
    local ${__helm_override_flag_list[*]##*-} two_word_of of var
    for w in "${words[@]}"; do
        if [ -n "${two_word_of}" ]; then
            eval "${two_word_of##*-}=\"${two_word_of}=\${w}\""
            two_word_of=
            continue
        fi
        for of in "${__helm_override_flag_list[@]}"; do
            case "${w}" in
                ${of}=*)
                    eval "${of##*-}=\"${w}\""
                    ;;
                ${of})
                    two_word_of="${of}"
                    ;;
            esac
        done
    done
    for var in "${__helm_override_flag_list[@]##*-}"; do
        if eval "test -n \"\$${var}\""; then
            eval "echo \${${var}}"
        fi
    done
}

__helm_binary_name()
{
    local helm_binary
    helm_binary="${words[0]}"
    __helm_debug "${FUNCNAME[0]}: helm_binary is ${helm_binary}"
    echo ${helm_binary}
}

__helm_list_releases()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local out filter
    # Use ^ to map from the start of the release name
    filter="^${words[c]}"
    # Use eval in case helm_binary_name or __helm_override_flags contains a variable (e.g., $HOME/bin/h2)
    if out=$(eval $(__helm_binary_name) list $(__helm_override_flags) -a -q -m 1000 ${filter} 2>/dev/null); then
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_list_repos()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local out oflags
    oflags=$(__helm_override_flags)
    __helm_debug "${FUNCNAME[0]}: __helm_override_flags are ${oflags}"
    # Use eval in case helm_binary_name contains a variable (e.g., $HOME/bin/h2)
    if out=$(eval $(__helm_binary_name) repo list ${oflags} 2>/dev/null | tail +2 | cut -f1); then
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_list_plugins()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    local out oflags
    oflags=$(__helm_override_flags)
    __helm_debug "${FUNCNAME[0]}: __helm_override_flags are ${oflags}"
    # Use eval in case helm_binary_name contains a variable (e.g., $HOME/bin/h2)
    if out=$(eval $(__helm_binary_name) plugin list ${oflags} 2>/dev/null | tail +2 | cut -f1); then
        COMPREPLY=( $( compgen -W "${out[*]}" -- "$cur" ) )
    fi
}

__helm_custom_func()
{
    __helm_debug "${FUNCNAME[0]}: c is $c words[@] is ${words[@]}"
    case ${last_command} in
        helm_delete | helm_history | helm_status | helm_test |\
        helm_upgrade | helm_rollback | helm_get_*)
            __helm_list_releases
            return
            ;;
        helm_repo_remove | helm_repo_update)
            __helm_list_repos
            return
            ;;
        helm_plugin_remove | helm_plugin_update)
            __helm_list_plugins
            return
            ;;
        *)
            ;;
    esac
}
`
)

var (
	tillerTunnel *kube.Tunnel
	settings     helm_env.EnvSettings
)

var globalUsage = `The Kubernetes package manager

To begin working with Helm, run the 'helm init' command:

	$ helm init

This will install Tiller to your running Kubernetes cluster.
It will also set up any necessary local configuration.

Common actions from this point include:

- helm search:    Search for charts
- helm fetch:     Download a chart to your local directory to view
- helm install:   Upload the chart to Kubernetes
- helm list:      List releases of charts

Environment:

- $HELM_HOME:           Set an alternative location for Helm files. By default, these are stored in ~/.helm
- $HELM_HOST:           Set an alternative Tiller host. The format is host:port
- $HELM_NO_PLUGINS:     Disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.
- $TILLER_NAMESPACE:    Set an alternative Tiller namespace (default "kube-system")
- $KUBECONFIG:          Set an alternative Kubernetes configuration file (default "~/.kube/config")
- $HELM_TLS_CA_CERT:    Path to TLS CA certificate used to verify the Helm client and Tiller server certificates (default "$HELM_HOME/ca.pem")
- $HELM_TLS_CERT:       Path to TLS client certificate file for authenticating to Tiller (default "$HELM_HOME/cert.pem")
- $HELM_TLS_KEY:        Path to TLS client key file for authenticating to Tiller (default "$HELM_HOME/key.pem")
- $HELM_TLS_ENABLE:     Enable TLS connection between Helm and Tiller (default "false")
- $HELM_TLS_VERIFY:     Enable TLS connection between Helm and Tiller and verify Tiller server certificate (default "false")
- $HELM_TLS_HOSTNAME:   The hostname or IP address used to verify the Tiller server certificate (default "127.0.0.1")
- $HELM_KEY_PASSPHRASE: Set HELM_KEY_PASSPHRASE to the passphrase of your PGP private key. If set, you will not be prompted for the passphrase while signing helm charts

`

func newRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		Long:         globalUsage,
		SilenceUsage: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			if settings.TLSCaCertFile == helm_env.DefaultTLSCaCert || settings.TLSCaCertFile == "" {
				settings.TLSCaCertFile = settings.Home.TLSCaCert()
			} else {
				settings.TLSCaCertFile = os.ExpandEnv(settings.TLSCaCertFile)
			}
			if settings.TLSCertFile == helm_env.DefaultTLSCert || settings.TLSCertFile == "" {
				settings.TLSCertFile = settings.Home.TLSCert()
			} else {
				settings.TLSCertFile = os.ExpandEnv(settings.TLSCertFile)
			}
			if settings.TLSKeyFile == helm_env.DefaultTLSKeyFile || settings.TLSKeyFile == "" {
				settings.TLSKeyFile = settings.Home.TLSKey()
			} else {
				settings.TLSKeyFile = os.ExpandEnv(settings.TLSKeyFile)
			}
		},
		PersistentPostRun: func(*cobra.Command, []string) {
			teardown()
		},
		BashCompletionFunction: bashCompletionFunc,
	}
	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)

	out := cmd.OutOrStdout()

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
		newServeCmd(out),
		newVerifyCmd(out),

		// release commands
		newDeleteCmd(nil, out),
		newGetCmd(nil, out),
		newHistoryCmd(nil, out),
		newInstallCmd(nil, out),
		newListCmd(nil, out),
		newRollbackCmd(nil, out),
		newStatusCmd(nil, out),
		newUpgradeCmd(nil, out),

		newReleaseTestCmd(nil, out),
		newResetCmd(nil, out),
		newVersionCmd(nil, out),

		newCompletionCmd(out),
		newHomeCmd(out),
		newInitCmd(out),
		newPluginCmd(out),
		newTemplateCmd(out),

		// Hidden documentation generator command: 'helm docs'
		newDocsCmd(out),

		// Deprecated
		markDeprecated(newRepoUpdateCmd(out), "Use 'helm repo update'\n"),
	)

	flags.Parse(args)

	// set defaults from environment
	settings.Init(flags)

	// Find and add plugins
	loadPlugins(cmd, out)

	return cmd
}

func init() {
	// Tell gRPC not to log to console.
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
}

func main() {
	cmd := newRootCmd(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		switch e := err.(type) {
		case pluginError:
			os.Exit(e.code)
		default:
			os.Exit(1)
		}
	}
}

func markDeprecated(cmd *cobra.Command, notice string) *cobra.Command {
	cmd.Deprecated = notice
	return cmd
}

func setupConnection() error {
	if settings.TillerHost == "" {
		config, client, err := getKubeClient(settings.KubeContext, settings.KubeConfig)
		if err != nil {
			return err
		}

		tillerTunnel, err = portforwarder.New(settings.TillerNamespace, client, config)
		if err != nil {
			return err
		}

		settings.TillerHost = fmt.Sprintf("127.0.0.1:%d", tillerTunnel.Local)
		debug("Created tunnel using local port: '%d'\n", tillerTunnel.Local)
	}

	// Set up the gRPC config.
	debug("SERVER: %q\n", settings.TillerHost)

	// Plugin support.
	return nil
}

func teardown() {
	if tillerTunnel != nil {
		tillerTunnel.Close()
	}
}

func checkArgsLength(argsReceived int, requiredArgs ...string) error {
	expectedNum := len(requiredArgs)
	if argsReceived != expectedNum {
		arg := "arguments"
		if expectedNum == 1 {
			arg = "argument"
		}
		return fmt.Errorf("This command needs %v %s: %s", expectedNum, arg, strings.Join(requiredArgs, ", "))
	}
	return nil
}

// prettyError unwraps or rewrites certain errors to make them more user-friendly.
func prettyError(err error) error {
	// Add this check can prevent the object creation if err is nil.
	if err == nil {
		return nil
	}
	// If it's grpc's error, make it more user-friendly.
	if s, ok := status.FromError(err); ok {
		return fmt.Errorf(s.Message())
	}
	// Else return the original error.
	return err
}

// configForContext creates a Kubernetes REST client configuration for a given kubeconfig context.
func configForContext(context string, kubeconfig string) (*rest.Config, error) {
	config, err := kube.GetConfig(context, kubeconfig).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes config for context %q: %s", context, err)
	}
	return config, nil
}

// getKubeClient creates a Kubernetes config and client for a given kubeconfig context.
func getKubeClient(context string, kubeconfig string) (*rest.Config, kubernetes.Interface, error) {
	config, err := configForContext(context, kubeconfig)
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get Kubernetes client: %s", err)
	}
	return config, client, nil
}

// ensureHelmClient returns a new helm client impl. if h is not nil.
func ensureHelmClient(h helm.Interface) helm.Interface {
	if h != nil {
		return h
	}
	return newClient()
}

func newClient() helm.Interface {
	options := []helm.Option{helm.Host(settings.TillerHost), helm.ConnectTimeout(settings.TillerConnectionTimeout)}

	if settings.TLSVerify || settings.TLSEnable {
		debug("Host=%q, Key=%q, Cert=%q, CA=%q\n", settings.TLSServerName, settings.TLSKeyFile, settings.TLSCertFile, settings.TLSCaCertFile)
		tlsopts := tlsutil.Options{
			ServerName:         settings.TLSServerName,
			KeyFile:            settings.TLSKeyFile,
			CertFile:           settings.TLSCertFile,
			InsecureSkipVerify: true,
		}
		if settings.TLSVerify {
			tlsopts.CaCertFile = settings.TLSCaCertFile
			tlsopts.InsecureSkipVerify = false
		}
		tlscfg, err := tlsutil.ClientConfig(tlsopts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		options = append(options, helm.WithTLS(tlscfg))
	}
	return helm.NewClient(options...)
}
