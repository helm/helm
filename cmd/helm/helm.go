package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kubernetes/helm/pkg/helm"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const (
	homeEnvVar  = "HELM_HOME"
	defaultHome = "$HOME/.helm" // FIXME: is $HOME windows compatible?
	hostEnvVar  = "TILLER_HOST"
	defaultHost = ":44134"
)

var helmHome string
var tillerHost string

// flagVerbose is a signal that the user wants additional output.
var flagVerbose bool

var globalUsage = `The Kubernetes package manager

To begin working with Helm, run the 'helm init' command:

	$ helm init

This will install Tiller to your running Kubernetes cluster.
It will also set up any necessary local configuration.

Common actions from this point include:

- helm search: search for charts
- helm fetch: download a chart to your local directory to view
- helm install: upload the chart to Kubernetes
- helm list: list releases of charts

Environment:
  $HELM_HOME    Set an alternative location for Helm files. By default, these are stored in ~/.helm
  $TILLER_HOST  Set an alternative Tiller host. The format is host:port (default ":44134").
`

// RootCommand is the top-level command for Helm.
var RootCommand = &cobra.Command{
	Use:              "helm",
	Short:            "The Helm package manager for Kubernetes.",
	Long:             globalUsage,
	PersistentPreRun: bootstrap,
}

func init() {
	home := os.Getenv(homeEnvVar)
	if home == "" {
		home = "$HOME/.helm"
	}
	thost := os.Getenv(hostEnvVar)
	if thost == "" {
		thost = defaultHost
	}
	p := RootCommand.PersistentFlags()
	p.StringVar(&helmHome, "home", home, "location of your Helm config. Overrides $HELM_HOME.")
	p.StringVar(&tillerHost, "host", thost, "address of tiller. Overrides $TILLER_HOST.")
	p.BoolVarP(&flagVerbose, "verbose", "v", false, "enable verbose output")
}

func main() {
	if err := RootCommand.Execute(); err != nil {
		os.Exit(1)
	}
}

func bootstrap(c *cobra.Command, args []string) {
	// Set up the gRPC config.
	helm.Config.ServAddr = tillerHost
	if flagVerbose {
		fmt.Printf("Server: %q\n", helm.Config.ServAddr)
	}
}

func checkArgsLength(expectedNum, actualNum int, requiredArgs ...string) error {
	if actualNum != expectedNum {
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
	// This is ridiculous. Why is 'grpc.rpcError' not exported? The least they
	// could do is throw an interface on the lib that would let us get back
	// the desc. Instead, we have to pass ALL errors through this.
	return errors.New(grpc.ErrorDesc(err))
}
