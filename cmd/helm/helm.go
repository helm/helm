package main

import (
	"os"

	"github.com/spf13/cobra"
)

var stdout = os.Stdout

var RootCommand = &cobra.Command{
	Use:   "helm",
	Short: "The Helm package manager for Kubernetes.",
	Long:  `Do long help here.`,
}

func main() {
	RootCommand.Execute()
}
