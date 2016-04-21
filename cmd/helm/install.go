package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/deis/tiller/pkg/chart"
	"github.com/deis/tiller/pkg/helm"
)

const installDesc = `
This command installs a chart archive.
`

func init() {
	RootCommand.Flags()
	RootCommand.AddCommand(installCmd)
}

var installCmd = &cobra.Command{
	Use:   "install [CHART]",
	Short: "install a chart archive.",
	Long:  installDesc,
	RunE:  runInstall,
}

func runInstall(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("This command needs at least one argument, the name of the chart.")
	}

	ch, err := loadChart(args[0])
	if err != nil {
		return err
	}

	res, err := helm.InstallRelease(ch)
	if err != nil {
		return err
	}

	fmt.Printf("release.name:   %s\n", res.Release.Name)
	fmt.Printf("release.chart:  %s\n", res.Release.Chart.Metadata.Name)
	fmt.Printf("release.status: %s\n", res.Release.Info.Status.Code)

	return nil
}

func loadChart(path string) (*chart.Chart, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if fi, err := os.Stat(path); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return chart.LoadDir(path)
	}

	return chart.Load(path)
}
