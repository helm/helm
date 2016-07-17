/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main // import "k8s.io/helm/cmd/helm"

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const (
	homeEnvVar      = "HELM_HOME"
	hostEnvVar      = "HELM_HOST"
	tillerNamespace = "kube-system"
)

var (
	helmHome   string
	tillerHost string
)

// flagDebug is a signal that the user wants additional output.
var flagDebug bool

var globalUsage = `The Kubernetes package manager

To begin working with Helm, run the 'helm init' command:

	$ helm init

This will install Tiller to your running Kubernetes cluster.
It will also set up any necessary local configuration.

Common actions from this point include:

- helm search:    search for charts
- helm fetch:     download a chart to your local directory to view
- helm install:   upload the chart to Kubernetes
- helm list:      list releases of charts

Environment:
  $HELM_HOME      Set an alternative location for Helm files. By default, these are stored in ~/.helm
  $HELM_HOST      Set an alternative Tiller host. The format is host:port (default ":44134").
`

func newRootCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		Long:         globalUsage,
		SilenceUsage: true,
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			teardown()
		},
	}
	home := os.Getenv(homeEnvVar)
	if home == "" {
		home = "$HOME/.helm"
	}
	thost := os.Getenv(hostEnvVar)
	p := cmd.PersistentFlags()
	p.StringVar(&helmHome, "home", home, "location of your Helm config. Overrides $HELM_HOME.")
	p.StringVar(&tillerHost, "host", thost, "address of tiller. Overrides $HELM_HOST.")
	p.BoolVarP(&flagDebug, "debug", "", false, "enable verbose output")

	cmd.AddCommand(
		newCreateCmd(out),
		newGetCmd(nil, out),
		newListCmd(nil, out),
		newStatusCmd(nil, out),
		newInstallCmd(nil, out),
		newDeleteCmd(nil, out),
		newInspectCmd(nil, out),
		newUpgradeCmd(nil, out),
	)
	return cmd
}

// RootCommand is the top-level command for Helm.
var RootCommand = newRootCmd(os.Stdout)

func main() {
	cmd := RootCommand
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupConnection(c *cobra.Command, args []string) error {
	if tillerHost == "" {
		tunnel, err := newTillerPortForwarder(tillerNamespace)
		if err != nil {
			return err
		}

		tillerHost = fmt.Sprintf(":%d", tunnel.Local)
		if flagDebug {
			fmt.Printf("Created tunnel using local port: '%d'\n", tunnel.Local)
		}
	}

	// Set up the gRPC config.
	if flagDebug {
		fmt.Printf("Server: %q\n", tillerHost)
	}
	return nil
}

func teardown() {
	if tunnel != nil {
		tunnel.Close()
	}
}

func checkArgsLength(expectedNum, actualNum int, requiredArgs ...string) error {
	if actualNum != expectedNum {
		arg := "arguments"
		if expectedNum == 1 {
			arg = "argument"
		}
		return fmt.Errorf("This command needs %v %s: %s", expectedNum, arg, strings.Join(requiredArgs, ", "))
	}
	return nil
}

// requireInit is a PreRunE implementation for validating that $HELM_HOME is configured.
func requireInit(cmd *cobra.Command, args []string) error {
	err := requireHome()
	if err != nil {
		return fmt.Errorf("%s (try running 'helm init')", err)
	}
	return nil
}

// prettyError unwraps or rewrites certain errors to make them more user-friendly.
func prettyError(err error) error {
	if err == nil {
		return nil
	}
	// This is ridiculous. Why is 'grpc.rpcError' not exported? The least they
	// could do is throw an interface on the lib that would let us get back
	// the desc. Instead, we have to pass ALL errors through this.
	return errors.New(grpc.ErrorDesc(err))
}
