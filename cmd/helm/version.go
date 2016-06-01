package main

import (
	"fmt"

	"github.com/kubernetes/helm/pkg/version"

	"github.com/spf13/cobra"
)

func init() {
	RootCommand.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the client version information.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.Version)
	},
}
