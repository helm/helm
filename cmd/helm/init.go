package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	RootCommand.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Helm on both client and server.",
	Long:  `Add long help here`,
	Run:   runInit,
}

func runInit(cmd *cobra.Command, args []string) {
	fmt.Fprintln(stdout, "Init was called.")
}
