package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	versionHelp = "This command prints out the current version of the Helm CLI."
	// this variable is set by the Makefile, using the Go linker flags
	cliVersion = "devel"
)

var versionCommand = &cobra.Command{
	Use:     "version",
	Short:   "Get the current version of Helm",
	Long:    versionHelp,
	RunE:    versionCmd,
	Aliases: []string{"vsn"},
}

func init() {
	RootCommand.AddCommand(versionCommand)
}

func versionCmd(cmd *cobra.Command, args []string) error {
	fmt.Println("Helm CLI version", cliVersion)
	return nil
}
