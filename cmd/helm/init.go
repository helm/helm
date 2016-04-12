package main

import (
	"errors"
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
	RunE:  RunInit,
}

// RunInit initializes local config and installs tiller to Kubernetes Cluster
func RunInit(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(stdout, "Init was called.")
	return errors.New("NotImplemented")
}
