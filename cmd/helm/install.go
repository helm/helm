package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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

// install flags & args
var (
	// installArg is the name or relative path of the chart to install
	installArg string
	// installDryRun performs a dry-run install
	installDryRun bool
	// installValues is the filename of supplied values.
	installValues string
	// installRelName is the user-supplied release name.
	installRelName string
)

var installCmd = &cobra.Command{
	Use:   "install [CHART]",
	Short: "install a chart archive.",
	Long:  installDesc,
	RunE:  runInstall,
}

func init() {
	f := installCmd.Flags()
	f.StringVarP(&installValues, "values", "f", "", "path to a values TOML file")
	f.StringVarP(&installRelName, "name", "n", "", "the release name. If unspecified, it will autogenerate one for you.")
	f.BoolVar(&installDryRun, "dry-run", false, "simulate an install")

	RootCommand.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	if err := checkArgsLength(1, len(args), "chart name"); err != nil {
		return err
	}
	chartpath, err := locateChartPath(args[0])
	if err != nil {
		return err
	}
	if flagVerbose {
		fmt.Printf("Chart path: %s\n", chartpath)
	}

	rawVals, err := vals()
	if err != nil {
		return err
	}

	res, err := helm.InstallRelease(rawVals, installRelName, chartpath, installDryRun)
	if err != nil {
		return prettyError(err)
	}

	printRelease(res.GetRelease())

	return nil
}

func vals() ([]byte, error) {
	if installValues == "" {
		return []byte{}, nil
	}
	return ioutil.ReadFile(installValues)
}

func printRelease(rel *release.Release) {
	if rel == nil {
		return
	}
	if flagVerbose {
		fmt.Printf("NAME:   %s\n", rel.Name)
		fmt.Printf("INFO:   %s %s\n", timeconv.String(rel.Info.LastDeployed), rel.Info.Status)
		fmt.Printf("CHART:  %s %s\n", rel.Chart.Metadata.Name, rel.Chart.Metadata.Version)
		fmt.Printf("MANIFEST: %s\n", rel.Manifest)
	} else {
		fmt.Println(rel.Name)
	}
}

// locateChartPath looks for a chart directory in known places, and returns either the full path or an error.
//
// This does not ensure that the chart is well-formed; only that the requested filename exists.
//
// Order of resolution:
// - current working directory
// - if path is absolute or begins with '.', error out here
// - chart repos in $HELM_HOME
func locateChartPath(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		return filepath.Abs(name)
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, fmt.Errorf("path %q not found", name)
	}

	crepo := filepath.Join(repositoryDirectory(), name)
	if _, err := os.Stat(crepo); err == nil {
		return filepath.Abs(crepo)
	}
	return name, fmt.Errorf("file %q not found", name)
}
