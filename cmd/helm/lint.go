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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/lint"
)

var longLintHelp = `
This command takes a path to a chart and runs a series of tests to verify that
the chart is well-formed.

If the linter encounters things that will cause the chart to fail installation,
it will emit [ERROR] messages. If it encounters issues that break with convention
or recommendation, it will emit [WARNING] messages.
`

var lintCommand = &cobra.Command{
	Use:   "lint [flags] PATH",
	Short: "examines a chart for possible issues",
	Long:  longLintHelp,
	RunE:  lintCmd,
}

func init() {
	RootCommand.AddCommand(lintCommand)
}

var errLintNoChart = errors.New("no chart found for linting (missing Chart.yaml)")

func lintCmd(cmd *cobra.Command, args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	// Guard: Error out of this is not a chart.
	if _, err := os.Stat(filepath.Join(path, "Chart.yaml")); err != nil {
		return errLintNoChart
	}

	issues := lint.All(path)

	if len(issues) == 0 {
		fmt.Println("Lint OK")
	}

	for _, i := range issues {
		fmt.Printf("%s\n", i)
	}
	return nil
}
