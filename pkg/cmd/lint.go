/*
Copyright The Helm Authors.

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

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
	"helm.sh/helm/v4/pkg/cli/output"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/getter"
)

var longLintHelp = `
This command takes a path to a chart and runs a series of tests to verify that
the chart is well-formed.

If the linter encounters things that will cause the chart to fail installation,
it will emit [ERROR] messages. If it encounters issues that break with convention
or recommendation, it will emit [WARNING] messages.
`

func newLintCmd(out io.Writer) *cobra.Command {
	client := action.NewLint()
	valueOpts := &values.Options{}
	var kubeVersion string
	var outfmt output.Format

	cmd := &cobra.Command{
		Use:   "lint PATH",
		Short: "examine a chart for possible issues",
		Long:  longLintHelp,
		Args:  require.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			paths := args

			if kubeVersion != "" {
				parsedKubeVersion, err := common.ParseKubeVersion(kubeVersion)
				if err != nil {
					return fmt.Errorf("invalid kube version '%s': %w", kubeVersion, err)
				}
				client.KubeVersion = parsedKubeVersion
			}

			if client.WithSubcharts {
				for _, p := range paths {
					filepath.Walk(filepath.Join(p, "charts"), func(path string, info os.FileInfo, _ error) error {
						if info != nil {
							if info.Name() == "Chart.yaml" {
								paths = append(paths, filepath.Dir(path))
							} else if strings.HasSuffix(path, ".tgz") || strings.HasSuffix(path, ".tar.gz") {
								paths = append(paths, path)
							}
						}
						return nil
					})
				}
			}

			client.Namespace = settings.Namespace()
			vals, err := valueOpts.MergeValues(getter.All(settings))
			if err != nil {
				return err
			}

			lw := &lintWriter{ChartsLinted: len(paths), Results: []lintChartResult{}}
			errorsOrWarnings := 0

			for _, path := range paths {
				result := client.Run([]string{path}, vals)

				// If there is no errors/warnings and quiet flag is set
				// go to the next chart
				hasWarningsOrErrors := action.HasWarningsOrErrors(result)
				if hasWarningsOrErrors {
					errorsOrWarnings++
				}
				if client.Quiet && !hasWarningsOrErrors {
					continue
				}

				lw.addResult(path, result, client.Quiet)

				if len(result.Errors) != 0 {
					lw.ChartsFailed++
				}
			}

			if err := outfmt.Write(out, lw); err != nil {
				return err
			}

			summary := fmt.Sprintf("%d chart(s) linted, %d chart(s) failed", lw.ChartsLinted, lw.ChartsFailed)
			if lw.ChartsFailed > 0 {
				return errors.New(summary)
			}
			if outfmt == output.Table && (!client.Quiet || errorsOrWarnings > 0) {
				fmt.Fprintln(out, summary)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&client.Strict, "strict", false, "fail on lint warnings")
	f.BoolVar(&client.WithSubcharts, "with-subcharts", false, "lint dependent charts")
	f.BoolVar(&client.Quiet, "quiet", false, "print only warnings and errors")
	f.BoolVar(&client.SkipSchemaValidation, "skip-schema-validation", false, "if set, disables JSON schema validation")
	f.StringVar(&kubeVersion, "kube-version", "", "Kubernetes version used for capabilities and deprecation checks")
	addValueOptionsFlags(f, valueOpts)
	bindOutputFlag(cmd, &outfmt)

	return cmd
}

// lintMessage is a single lint finding in machine-readable form.
type lintMessage struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

// lintChartResult holds the findings for one linted chart path.
type lintChartResult struct {
	Chart    string        `json:"chart"`
	Failed   bool          `json:"failed"`
	Messages []lintMessage `json:"messages"`
	Errors   []string      `json:"errors,omitempty"`
}

// lintWriter renders lint results as a human-readable table, JSON or YAML.
type lintWriter struct {
	ChartsLinted int               `json:"charts_linted"`
	ChartsFailed int               `json:"charts_failed"`
	Results      []lintChartResult `json:"results"`
}

// addResult records the outcome of linting a single chart path. The quiet flag
// mirrors the table behavior: with --quiet, informational messages and clean
// charts are skipped.
func (w *lintWriter) addResult(path string, result *action.LintResult, quiet bool) {
	entry := lintChartResult{Chart: path, Messages: []lintMessage{}}

	// All the Errors that are generated by a chart
	// that failed a lint will be included in the
	// results.Messages so we only need to include
	// the Errors if there are no Messages.
	if len(result.Messages) == 0 {
		for _, err := range result.Errors {
			entry.Errors = append(entry.Errors, err.Error())
		}
	}

	for _, msg := range result.Messages {
		if !quiet || msg.Severity > support.InfoSev {
			entry.Messages = append(entry.Messages, lintMessage{
				Severity: support.SeverityName(msg.Severity),
				Path:     msg.Path,
				Message:  msg.Err.Error(),
			})
		}
	}

	entry.Failed = len(result.Errors) != 0
	w.Results = append(w.Results, entry)
}

func (w *lintWriter) WriteTable(out io.Writer) error {
	var message strings.Builder

	for _, entry := range w.Results {
		fmt.Fprintf(&message, "==> Linting %s\n", entry.Chart)

		for _, err := range entry.Errors {
			fmt.Fprintf(&message, "Error %s\n", err)
		}

		for _, msg := range entry.Messages {
			fmt.Fprintf(&message, "[%s] %s: %s\n", msg.Severity, msg.Path, msg.Message)
		}

		// Adding extra new line here to break up the
		// results, stops this from being a big wall of
		// text and makes it easier to follow.
		fmt.Fprint(&message, "\n")
	}

	fmt.Fprint(out, message.String())
	return nil
}

func (w *lintWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, w)
}

func (w *lintWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, w)
}
