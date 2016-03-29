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

package main

import (
	"errors"
	"fmt"

	"github.com/codegangsta/cli"
	"github.com/kubernetes/helm/pkg/client"
	"github.com/kubernetes/helm/pkg/format"
	"github.com/kubernetes/helm/pkg/kubectl"
)

// ErrAlreadyInstalled indicates that Helm Server is already installed.
var ErrAlreadyInstalled = errors.New("Already Installed")

func init() {
	addCommands(dmCmd())
}

func dmCmd() cli.Command {
	return cli.Command{
		Name:  "server",
		Usage: "Manage Helm server-side components",
		Description: `Server commands manage the in-cluster portion of Helm.

   Helm  has several components that run inside of Kubernetes. Before Helm can
   be used to install and manage packages, it must be installed into the
   Kubernetes cluster in which packages will be installed.

   The 'helm server' commands rely upon a properly configured 'kubectl' to
   communicate with the Kubernetes cluster. To verify that your 'kubectl'
   client is pointed to the correct cluster, use 'kubectl cluster-info'.

   Use 'helm server install' to install the in-cluster portion of Helm.
`,
		Subcommands: []cli.Command{
			{
				Name:      "install",
				Usage:     "Install Helm server components on Kubernetes.",
				ArgsUsage: "",
				Description: `Use kubectl to install Helm components in their own namespace on Kubernetes.

	Make sure your Kubernetes environment is pointed to the cluster on which you
	wish to install.`,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Show what would be installed, but don't install anything.",
					},
					cli.StringFlag{
						Name:   "resourcifier-image",
						Usage:  "The full image name of the Docker image for resourcifier.",
						EnvVar: "HELM_RESOURCIFIER_IMAGE",
					},
					cli.StringFlag{
						Name:   "expandybird-image",
						Usage:  "The full image name of the Docker image for expandybird.",
						EnvVar: "HELM_EXPANDYBIRD_IMAGE",
					},
					cli.StringFlag{
						Name:   "manager-image",
						Usage:  "The full image name of the Docker image for manager.",
						EnvVar: "HELM_MANAGER_IMAGE",
					},
				},
				Action: func(c *cli.Context) { run(c, installServer) },
			},
			{
				Name:        "uninstall",
				Usage:       "Uninstall the Helm server-side from Kubernetes.",
				ArgsUsage:   "",
				Description: ``,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Show what would be uninstalled, but don't remove anything.",
					},
				},
				Action: func(c *cli.Context) { run(c, uninstallServer) },
			},
			{
				Name:      "status",
				Usage:     "Show status of Helm server-side components.",
				ArgsUsage: "",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Only display the underlying kubectl commands.",
					},
				},
				Action: func(c *cli.Context) { run(c, statusServer) },
			},
			{
				Name:      "target",
				Usage:     "Displays information about the Kubernetes cluster.",
				ArgsUsage: "",
				Action:    func(c *cli.Context) { run(c, targetServer) },
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Only display the underlying kubectl commands.",
					},
				},
			},
		},
	}
}

func installServer(c *cli.Context) error {
	resImg := c.String("resourcifier-image")
	ebImg := c.String("expandybird-image")
	manImg := c.String("manager-image")

	dryRun := c.Bool("dry-run")
	kubectlPath := c.String("kubectl")
	runner := buildKubectlRunner(kubectlPath, dryRun)

	i := client.NewInstaller()
	i.Manager["Image"] = manImg
	i.Resourcifier["Image"] = resImg
	i.Expandybird["Image"] = ebImg

	out, err := i.Install(runner)
	if err != nil {
		return fmt.Errorf("error installing %s %s", string(out), err)
	}
	format.Msg(out)
	return nil
}

func uninstallServer(c *cli.Context) error {
	dryRun := c.Bool("dry-run")
	kubectlPath := c.String("kubectl")
	runner := buildKubectlRunner(kubectlPath, dryRun)

	out, err := client.Uninstall(runner)
	if err != nil {
		return fmt.Errorf("error uninstalling: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func statusServer(c *cli.Context) error {
	dryRun := c.Bool("dry-run")
	kubectlPath := c.String("kubectl")
	runner := buildKubectlRunner(kubectlPath, dryRun)

	out, err := runner.GetByKind("pods", "", "dm")
	if err != nil {
		return err
	}
	format.Msg(string(out))
	return nil
}

func targetServer(c *cli.Context) error {
	dryRun := c.Bool("dry-run")
	kubectlPath := c.String("kubectl")
	runner := buildKubectlRunner(kubectlPath, dryRun)

	out, err := runner.ClusterInfo()
	if err != nil {
		return fmt.Errorf("%s (%s)", out, err)
	}
	format.Msg(string(out))
	return nil
}

func buildKubectlRunner(kubectlPath string, dryRun bool) kubectl.Runner {
	if dryRun {
		return &kubectl.PrintRunner{}
	}
	// TODO: Refactor out kubectl.Path global
	if kubectlPath != "" {
		kubectl.Path = kubectlPath
	}
	return &kubectl.RealRunner{}
}
