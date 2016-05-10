package main

import (
	"fmt"

	"github.com/kubernetes/helm/pkg/helm"
	"github.com/kubernetes/helm/pkg/timeconv"
	"github.com/spf13/cobra"
)

var statusHelp = `
This command shows the status of a named release.
`

var statusCommand = &cobra.Command{
	Use:   "status [flags] RELEASE_NAME",
	Short: "Displays the status of the named release",
	Long:  statusHelp,
	RunE:  status,
}

func init() {
	RootCommand.AddCommand(statusCommand)
}

func status(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errReleaseRequired
	}

	res, err := helm.GetReleaseStatus(args[0])
	if err != nil {
		return prettyError(err)
	}

	fmt.Printf("Last Deployed: %s\n", timeconv.String(res.Info.LastDeployed))
	fmt.Printf("Status: %s\n", res.Info.Status.Code)
	if res.Info.Status.Details != nil {
		fmt.Printf("Details: %s\n", res.Info.Status.Details)
	}

	return nil
}
