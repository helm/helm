package main

import (
	"fmt"

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
	Run:   Home,
}

func init() {
	RootCommand.AddCommand(homeCommand)
}

func Home(cmd *cobra.Command, args []string) {
	fmt.Println("helm home was called")
}
