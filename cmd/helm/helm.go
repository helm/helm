package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var stdout = os.Stdout
var helmHome string

var globalUsage = `The Kubernetes package manager

To begin working with Helm, run the 'helm init' command:

$ helm init

This will install Tiller to your running Kubernetes cluster.
It will also set up any necessary local configuration.

Commond actions from this point on include:

- helm search: search for charts
- helm fetch: download a chart to your local directory to view
- helm install: upload the chart to Kubernetes
- helm list: list releases of charts

ENVIRONMENT:
$HELM_HOME: Set an alternative location for Helm files.
            By default, these are stored in ~/.helm
`

// RootCommand is the top-level command for Helm.
var RootCommand = &cobra.Command{
	Use:   "helm",
	Short: "The Helm package manager for Kubernetes.",
	Long:  globalUsage,
}

func init() {
	RootCommand.PersistentFlags().StringVar(&helmHome, "home", "$HOME/.helm", "location of you Helm files [$HELM_HOME]")
}

func main() {
	RootCommand.Execute()
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
