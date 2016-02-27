package main

import (
	"errors"
	"os"

	"github.com/codegangsta/cli"
	"github.com/deis/helm-dm/dm"
	"github.com/deis/helm-dm/format"
	"github.com/deis/helm-dm/kubectl"
)

// ErrAlreadyInstalled indicates that DM is already installed.
var ErrAlreadyInstalled = errors.New("Already Installed")

func init() {
	addCommands(dmCmd())
}

func dmCmd() cli.Command {
	return cli.Command{
		Name:  "dm",
		Usage: "Manage DM on Kubernetes",
		Subcommands: []cli.Command{
			{
				Name:        "install",
				Usage:       "Install DM on Kubernetes.",
				ArgsUsage:   "",
				Description: ``,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Show what would be installed, but don't install anything.",
					},
				},
				Action: func(c *cli.Context) {
					if err := install(c.Bool("dry-run")); err != nil {
						format.Err("%s (Run 'helm doctor' for more information)", err)
						os.Exit(1)
					}
				},
			},
			{
				Name:        "uninstall",
				Usage:       "Uninstall the DM from Kubernetes.",
				ArgsUsage:   "",
				Description: ``,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "dry-run",
						Usage: "Show what would be installed, but don't install anything.",
					},
				},
				Action: func(c *cli.Context) {
					if err := uninstall(c.Bool("dry-run")); err != nil {
						format.Err("%s (Run 'helm doctor' for more information)", err)
						os.Exit(1)
					}
				},
			},
			{
				Name:      "status",
				Usage:     "Show status of DM.",
				ArgsUsage: "",
				Action: func(c *cli.Context) {
					format.Err("Not yet implemented")
					os.Exit(1)
				},
			},
			{
				Name:      "target",
				Usage:     "Displays information about cluster.",
				ArgsUsage: "",
				Action: func(c *cli.Context) {
					if err := target(c.Bool("dry-run")); err != nil {
						format.Err("%s (Is the cluster running?)", err)
						os.Exit(1)
					}
				},
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

func install(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Install(runner)
	if err != nil {
		format.Err("Error installing: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func uninstall(dryRun bool) error {
	runner := getKubectlRunner(dryRun)

	out, err := dm.Uninstall(runner)
	if err != nil {
		format.Err("Error uninstalling: %s %s", out, err)
	}
	format.Msg(out)
	return nil
}

func getKubectlRunner(dryRun bool) kubectl.Runner {
	if dryRun {
		return &kubectl.PrintRunner{}
	}
	return &kubectl.RealRunner{}
}
