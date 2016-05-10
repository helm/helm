package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubernetes/helm/pkg/helm"
	"github.com/kubernetes/helm/pkg/proto/hapi/release"
	"github.com/kubernetes/helm/pkg/timeconv"
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
	// installArg is the name or relative path of the chart to install
	installArg string
	// tillerHost overrides TILLER_HOST envVar
	tillerHost string
	// installDryRun performs a dry-run install
	installDryRun bool
)

var installCmd = &cobra.Command{
	Use:   "install [CHART]",
	Short: "install a chart archive.",
	Long:  installDesc,
	RunE:  runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
	setupInstallEnv(args)

	res, err := helm.InstallRelease(installArg, installDryRun)
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
	if rel == nil {
		return
	}
	if flagVerbose {
		fmt.Printf("NAME:   %s\n", rel.Name)
		fmt.Printf("INFO:   %s %s\n", timeconv.Format(rel.Info.LastDeployed, time.ANSIC), rel.Info.Status)
		fmt.Printf("CHART:  %s %s\n", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
		fmt.Printf("MANIFEST: %s\n", rel.Manifest)
	} else {
		fmt.Println(rel.Name)
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
	installCmd.Flags().BoolVar(&installDryRun, "dry-run", false, "simulate an install")

	RootCommand.AddCommand(installCmd)
}
