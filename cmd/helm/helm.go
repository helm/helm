package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var helmHome string

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
`

// RootCommand is the top-level command for Helm.
var RootCommand = &cobra.Command{
	Use:   "helm",
	Short: "The Helm package manager for Kubernetes.",
	Long:  globalUsage,
}

func init() {
	RootCommand.PersistentFlags().StringVar(&helmHome, "home", "$HOME/.helm", "location of you Helm files [$HELM_HOME]")
	RootCommand.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "enable verbose output")
}

func main() {
	if err := RootCommand.Execute(); err != nil {
		os.Exit(1)
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
