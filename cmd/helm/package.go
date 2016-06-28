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
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/repo"
)

const packageDesc = `
This command packages a chart into a versioned chart archive file. If a path
is given, this will look at that path for a chart (which must contain a
Chart.yaml file) and then package that directory.

If no path is given, this will look in the present working directory for a
Chart.yaml file, and (if found) build the current directory into a chart.

Versioned chart archives are used by Helm package repositories.
`

var save bool

func init() {
	packageCmd.Flags().BoolVar(&save, "save", true, "save packaged chart to local chart repository")
	RootCommand.AddCommand(packageCmd)
}

var packageCmd = &cobra.Command{
	Use:   "package [CHART_PATH]",
	Short: "package a chart directory into a chart archive",
	Long:  packageDesc,
	RunE:  runPackage,
}

func runPackage(cmd *cobra.Command, args []string) error {
	path := "."

	if len(args) > 0 {
		path = args[0]
	} else {
		return fmt.Errorf("This command needs at least one argument, the path to the chart.")
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	ch, err := chartutil.LoadDir(path)
	if err != nil {
		return err
	}

	if filepath.Base(path) != ch.Metadata.Name {
		return fmt.Errorf("directory name (%s) and Chart.yaml name (%s) must match", filepath.Base(path), ch.Metadata.Name)
	}

	// Save to the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name, err := chartutil.Save(ch, cwd)
	if err == nil && flagDebug {
		cmd.Printf("Saved %s to current directory\n", name)
	}

	// Save to $HELM_HOME/local directory. This is second, because we don't want
	// the case where we saved here, but didn't save to the default destination.
	if save {
		if err := repo.AddChartToLocalRepo(ch, localRepoDirectory()); err != nil {
			return err
		} else if flagDebug {
			cmd.Printf("Saved %s to %s\n", name, localRepoDirectory())
		}
	}

	return err
}
