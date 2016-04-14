package main

import (
	"os"

	"github.com/spf13/cobra"
)

var stdout = os.Stdout

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

var RootCommand = &cobra.Command{
	Use:   "helm",
	Short: "The Helm package manager for Kubernetes.",
	Long:  globalUsage,
}

func main() {
	RootCommand.Execute()
}
