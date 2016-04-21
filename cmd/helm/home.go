package main

import (
	"os"

	"github.com/spf13/cobra"
)

var longHomeHelp = `
This command displays the location of HELM_HOME. This is where
any helm configuration files live.
`

var homeCommand = &cobra.Command{
	Use:   "home",
	Short: "Displays the location of HELM_HOME",
	Long:  longHomeHelp,
	Run:   home,
}

func init() {
	RootCommand.AddCommand(homeCommand)
}

func home(cmd *cobra.Command, args []string) {
	cmd.Printf(os.ExpandEnv(helmHome) + "\n")
}
