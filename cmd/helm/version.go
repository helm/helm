package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/version"
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
