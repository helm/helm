package main

import (
	"errors"
	"fmt"

	"github.com/deis/tiller/pkg/client"
	"github.com/deis/tiller/pkg/kubectl"
	"github.com/spf13/cobra"
)

const installDesc = `
This command installs Tiller (the helm server side component) onto your
Kubernetes Cluster and sets up local configuration in $HELM_HOME (default: ~/.helm/)
`

var tillerImg string

func init() {
	initCmd.Flags().StringVarP(&tillerImg, "tiller-image", "i", "", "override tiller image")
	RootCommand.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Helm on both client and server.",
	Long:  installDesc,
	RunE:  RunInit,
}

// RunInit initializes local config and installs tiller to Kubernetes Cluster
func RunInit(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("This command does not accept arguments. \n")
	}

	// TODO: take value of global flag kubectl and pass that in
	runner := buildKubectlRunner("")

	i := client.NewInstaller()
	i.Tiller["Image"] = tillerImg

	out, err := i.Install(runner)
	if err != nil {
		return fmt.Errorf("error installing %s %s", string(out), err)
	}

	fmt.Printf("Tiller (the helm server side component) has been installed into your Kubernetes Cluster.\n")
	return nil
}

func buildKubectlRunner(kubectlPath string) kubectl.Runner {
	if kubectlPath != "" {
		kubectl.Path = kubectlPath
	}
	return &kubectl.RealRunner{}
}
