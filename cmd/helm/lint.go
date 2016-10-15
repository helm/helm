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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/lint"
	"k8s.io/helm/pkg/lint/support"
)

var longLintHelp = `
This command takes a path to a chart and runs a series of tests to verify that
the chart is well-formed.

If the linter encounters things that will cause the chart to fail installation,
it will emit [ERROR] messages. If it encounters issues that break with convention
or recommendation, it will emit [WARNING] messages.
`

type lintCmd struct {
	strict bool
	paths  []string
	out    io.Writer
}

func newLintCmd(out io.Writer) *cobra.Command {
	l := &lintCmd{
		paths: []string{"."},
		out:   out,
	}
	cmd := &cobra.Command{
		Use:   "lint [flags] PATH",
		Short: "examines a chart for possible issues",
		Long:  longLintHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				l.paths = args
			}
			return l.run()
		},
	}

	cmd.Flags().BoolVar(&l.strict, "strict", false, "fail on lint warnings")

	return cmd
}

var errLintNoChart = errors.New("No chart found for linting (missing Chart.yaml)")

func (l *lintCmd) run() error {
	var lowestTolerance int
	if l.strict {
		lowestTolerance = support.WarningSev
	} else {
		lowestTolerance = support.ErrorSev
	}

	var total int
	var failures int
	for _, path := range l.paths {
		if linter, err := lintChart(path); err != nil {
			fmt.Println("==> Skipping", path)
			fmt.Println(err)
		} else {
			fmt.Println("==> Linting", path)

			if len(linter.Messages) == 0 {
				fmt.Println("Lint OK")
			}

			for _, msg := range linter.Messages {
				fmt.Println(msg)
			}

			total = total + 1
			if linter.HighestSeverity >= lowestTolerance {
				failures = failures + 1
			}
		}
		fmt.Println("")
	}

	msg := fmt.Sprintf("%d chart(s) linted", total)
	if failures > 0 {
		return fmt.Errorf("%s, %d chart(s) failed", msg, failures)
	}

	fmt.Fprintf(l.out, "%s, no failures\n", msg)

	return nil
}

func lintChart(path string) (support.Linter, error) {
	var chartPath string
	linter := support.Linter{}

	if strings.HasSuffix(path, ".tgz") {
		tempDir, err := ioutil.TempDir("", "helm-lint")
		if err != nil {
			return linter, err
		}
		defer os.RemoveAll(tempDir)

		file, err := os.Open(path)
		if err != nil {
			return linter, err
		}
		defer file.Close()

		if err = chartutil.Expand(tempDir, file); err != nil {
			return linter, err
		}

		base := strings.Split(filepath.Base(path), "-")[0]
		chartPath = filepath.Join(tempDir, base)
	} else {
		chartPath = path
	}

	// Guard: Error out of this is not a chart.
	if _, err := os.Stat(filepath.Join(chartPath, "Chart.yaml")); err != nil {
		return linter, errLintNoChart
	}

	return lint.All(chartPath), nil
}
