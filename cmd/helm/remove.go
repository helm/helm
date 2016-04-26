package main

import (
	"errors"
	"fmt"

	"github.com/deis/tiller/pkg/helm"
	"github.com/spf13/cobra"
)

const removeDesc = `
This command takes a release name, and then deletes the release from Kubernetes.
It removes all of the resources associated with the last release of the chart.

Use the '--dry-run' flag to see which releases will be deleted without actually
deleting them.
`

var removeDryRun bool

var removeCommand = &cobra.Command{
	Use:        "remove [flags] RELEASE_NAME",
	Aliases:    []string{"rm"},
	SuggestFor: []string{"delete", "del"},
	Short:      "Given a release name, remove the release from Kubernetes",
	Long:       removeDesc,
	RunE:       rmRelease,
}

func init() {
	RootCommand.AddCommand(removeCommand)
	removeCommand.Flags().BoolVar(&removeDryRun, "dry-run", false, "Simulate action, but don't actually do it.")
}

func rmRelease(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("Command 'remove' requires a release name.")
	}

	// TODO: Handle dry run use case.
	if removeDryRun {
		fmt.Printf("Deleting %s\n", args[0])
		return nil
	}

	_, err := helm.UninstallRelease(args[0])
	if err != nil {
		return err
	}

	return nil
}
