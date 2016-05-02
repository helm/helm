package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubernetes/helm/pkg/helm"
	"github.com/kubernetes/helm/pkg/proto/hapi/release"
)

const installDesc = `
This command installs a chart archive.

The install argument must be either a relative
path to a chart directory or the name of a
chart in the current working directory.
`

const (
	hostEnvVar  = "TILLER_HOST"
	defaultHost = ":44134"
)

// install flags & args
var (
	installArg string // name or relative path of the chart to install
	tillerHost string // override TILLER_HOST envVar
	verbose    bool   // enable verbose install
)

var installCmd = &cobra.Command{
	Use:   "install [CHART]",
	Short: "install a chart archive.",
	Long:  installDesc,
	RunE:  runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
	setupInstallEnv(args)

	res, err := helm.InstallRelease(installArg)
	if err != nil {
		return err
	}

	printRelease(res.GetRelease())

	return nil
}

// TODO -- Display formatted description of install release status / info.
// 		   Might be friendly to wrap our proto model with pretty-printers.
//
func printRelease(rel *release.Release) {
	if verbose {
		if rel != nil {
			fmt.Printf("release.name:   %s\n", rel.Name)
			fmt.Printf("release.info:   %s\n", rel.GetInfo())
			fmt.Printf("release.chart:  %s\n", rel.GetChart())
		}
	}
}

func setupInstallEnv(args []string) {
	if len(args) > 0 {
		installArg = args[0]
	} else {
		fatalf("This command needs at least one argument, the name of the chart.")
	}

	// note: TILLER_HOST envvar is only
	// acknowledged iff the host flag
	// does not override the default.
	if tillerHost == defaultHost {
		host := os.Getenv(hostEnvVar)
		if host != "" {
			tillerHost = host
		}
	}

	helm.Config.ServAddr = tillerHost
}

func fatalf(format string, args ...interface{}) {
	fmt.Printf("fatal: %s\n", fmt.Sprintf(format, args...))
	os.Exit(0)
}

func init() {
	installCmd.Flags().StringVar(&tillerHost, "host", defaultHost, "address of tiller server")
	installCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose install")

	RootCommand.AddCommand(installCmd)
}
