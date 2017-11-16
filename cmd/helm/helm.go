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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/helm/pkg/helm"
	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/portforwarder"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/tlsutil"
)

var (
	tlsCaCertFile string // path to TLS CA certificate file
	tlsCertFile   string // path to TLS certificate file
	tlsKeyFile    string // path to TLS key file
	tlsVerify     bool   // enable TLS and verify remote certificates
	tlsEnable     bool   // enable TLS

	tlsCaCertDefault = "$HELM_HOME/ca.pem"
	tlsCertDefault   = "$HELM_HOME/cert.pem"
	tlsKeyDefault    = "$HELM_HOME/key.pem"

	tillerTunnel *kube.Tunnel
	settings     helm_env.EnvSettings
)

var globalUsage = `The Kubernetes package manager

To begin working with Helm, run the 'helm init' command:

	$ helm init

This will install Tiller to your running Kubernetes cluster.
It will also set up any necessary local configuration.

Common actions from this point include:

- helm search:    search for charts
- helm fetch:     download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment:
  $HELM_HOME          set an alternative location for Helm files. By default, these are stored in ~/.helm
  $HELM_HOST          set an alternative Tiller host. The format is host:port
  $HELM_NO_PLUGINS    disable plugins. Set HELM_NO_PLUGINS=1 to disable plugins.
  $TILLER_NAMESPACE   set an alternative Tiller namespace (default "kube-system")
  $KUBECONFIG         set an alternative Kubernetes configuration file (default "~/.kube/config")
`

func newRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		Long:         globalUsage,
		SilenceUsage: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			tlsCaCertFile = os.ExpandEnv(tlsCaCertFile)
			tlsCertFile = os.ExpandEnv(tlsCertFile)
			tlsKeyFile = os.ExpandEnv(tlsKeyFile)
		},
		PersistentPostRun: func(*cobra.Command, []string) {
			teardown()
		},
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
		addFlagsTLS(newDeleteCmd(nil, out)),
		addFlagsTLS(newGetCmd(nil, out)),
		addFlagsTLS(newHistoryCmd(nil, out)),
		addFlagsTLS(newInstallCmd(nil, out)),
		addFlagsTLS(newListCmd(nil, out)),
		addFlagsTLS(newRollbackCmd(nil, out)),
		addFlagsTLS(newStatusCmd(nil, out)),
		addFlagsTLS(newUpgradeCmd(nil, out)),

		addFlagsTLS(newReleaseTestCmd(nil, out)),
		addFlagsTLS(newResetCmd(nil, out)),
		addFlagsTLS(newVersionCmd(nil, out)),

		newCompletionCmd(out),
		newHomeCmd(out),
		newInitCmd(out),
		newPluginCmd(out),
		newTemplateCmd(out),

		// Hidden documentation generator command: 'helm docs'
		newDocsCmd(out),

		// Deprecated
		markDeprecated(newRepoUpdateCmd(out), "use 'helm repo update'\n"),
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
		os.Exit(1)
	}
}

func markDeprecated(cmd *cobra.Command, notice string) *cobra.Command {
	cmd.Deprecated = notice
	return cmd
}

func setupConnection(c *cobra.Command, args []string) error {
	if settings.TillerHost == "" {
		config, client, err := getKubeClient(settings.KubeContext)
		if err != nil {
			return err
		}

		tunnel, err := portforwarder.New(settings.TillerNamespace, client, config)
		if err != nil {
			return err
		}

		settings.TillerHost = fmt.Sprintf("127.0.0.1:%d", tunnel.Local)
		debug("Created tunnel using local port: '%d'\n", tunnel.Local)
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
	if err == nil {
		return nil
	}
	// This is ridiculous. Why is 'grpc.rpcError' not exported? The least they
	// could do is throw an interface on the lib that would let us get back
	// the desc. Instead, we have to pass ALL errors through this.
	return errors.New(grpc.ErrorDesc(err))
}

// configForContext creates a Kubernetes REST client configuration for a given kubeconfig context.
func configForContext(context string) (*rest.Config, error) {
	config, err := kube.GetConfig(context).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes config for context %q: %s", context, err)
	}
	return config, nil
}

// getKubeClient creates a Kubernetes config and client for a given kubeconfig context.
func getKubeClient(context string) (*rest.Config, kubernetes.Interface, error) {
	config, err := configForContext(context)
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get Kubernetes client: %s", err)
	}
	return config, client, nil
}

// getInternalKubeClient creates a Kubernetes config and an "internal" client for a given kubeconfig context.
//
// Prefer the similar getKubeClient if you don't need to use such an internal client.
func getInternalKubeClient(context string) (internalclientset.Interface, error) {
	config, err := configForContext(context)
	if err != nil {
		return nil, err
	}
	client, err := internalclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes client: %s", err)
	}
	return client, nil
}

// ensureHelmClient returns a new helm client impl. if h is not nil.
func ensureHelmClient(h helm.Interface) helm.Interface {
	if h != nil {
		return h
	}
	return newClient()
}

func newClient() helm.Interface {
	options := []helm.Option{helm.Host(settings.TillerHost)}

	if tlsVerify || tlsEnable {
		if tlsCaCertFile == "" {
			tlsCaCertFile = os.ExpandEnv(tlsCaCertDefault)
		}
		if tlsCertFile == "" {
			tlsCertFile = os.ExpandEnv(tlsCertDefault)
		}
		if tlsKeyFile == "" {
			tlsKeyFile = os.ExpandEnv(tlsKeyDefault)
		}
		debug("Key=%q, Cert=%q, CA=%q\n", tlsKeyFile, tlsCertFile, tlsCaCertFile)
		tlsopts := tlsutil.Options{KeyFile: tlsKeyFile, CertFile: tlsCertFile, InsecureSkipVerify: true}
		if tlsVerify {
			tlsopts.CaCertFile = tlsCaCertFile
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

// addFlagsTLS adds the flags for supporting client side TLS to the
// helm command (only those that invoke communicate to Tiller.)
func addFlagsTLS(cmd *cobra.Command) *cobra.Command {

	// add flags
	cmd.Flags().StringVar(&tlsCaCertFile, "tls-ca-cert", tlsCaCertDefault, "path to TLS CA certificate file")
	cmd.Flags().StringVar(&tlsCertFile, "tls-cert", tlsCertDefault, "path to TLS certificate file")
	cmd.Flags().StringVar(&tlsKeyFile, "tls-key", tlsKeyDefault, "path to TLS key file")
	cmd.Flags().BoolVar(&tlsVerify, "tls-verify", false, "enable TLS for request and verify remote")
	cmd.Flags().BoolVar(&tlsEnable, "tls", false, "enable TLS for request")
	return cmd
}
