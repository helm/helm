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
	"encoding/json"
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

Use -o json or -o yaml for a report with charts_linted, charts_failed, and
per-chart results. Each result includes the chart path, its failed status,
and messages with severity, message, and a path when available. With --quiet,
clean charts and informational messages are omitted from results. The counts
include every chart checked.
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

			lw := &lintWriter{Results: []lintChartResult{}}
			for _, path := range paths {
				lw.addResult(path, client.Run([]string{path}, vals), client.Quiet)
			}
			if err := outfmt.Write(out, lw); err != nil {
				return err
			}
			if lw.ChartsFailed > 0 {
				return errors.New(lw.summary())
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

// lintMessages keeps each finding in its native form for text output.
type lintMessages []support.Message

func (messages lintMessages) MarshalJSON() ([]byte, error) {
	type finding struct {
		Severity string `json:"severity"`
		Path     string `json:"path,omitempty"`
		Message  string `json:"message"`
	}
	findings := make([]finding, 0, len(messages))
	for _, msg := range messages {
		findings = append(findings, finding{
			Severity: support.SeverityName(msg.Severity),
			Path:     msg.Path,
			Message:  msg.Err.Error(),
		})
	}
	return json.Marshal(findings)
}

type lintChartResult struct {
	Chart    string       `json:"chart"`
	Failed   bool         `json:"failed"`
	Messages lintMessages `json:"messages"`

	// Load errors use the historical "Error ..." prefix in text output.
	loadFailed bool
}

type lintWriter struct {
	ChartsLinted int               `json:"charts_linted"`
	ChartsFailed int               `json:"charts_failed"`
	Results      []lintChartResult `json:"results"`
}

func (w *lintWriter) addResult(path string, result *action.LintResult, quiet bool) {
	w.ChartsLinted++
	if quiet && !action.HasWarningsOrErrors(result) {
		return
	}

	entry := lintChartResult{Chart: path, Failed: len(result.Errors) > 0}
	if entry.Failed {
		w.ChartsFailed++
	}
	if entry.Failed && len(result.Messages) == 0 {
		entry.loadFailed = true
		for _, err := range result.Errors {
			entry.Messages = append(entry.Messages, support.NewMessage(support.ErrorSev, "", err))
		}
	} else {
		for _, msg := range result.Messages {
			if !quiet || msg.Severity > support.InfoSev {
				entry.Messages = append(entry.Messages, msg)
			}
		}
	}
	w.Results = append(w.Results, entry)
}

func (w *lintWriter) summary() string {
	return fmt.Sprintf("%d chart(s) linted, %d chart(s) failed", w.ChartsLinted, w.ChartsFailed)
}

func (w *lintWriter) WriteTable(out io.Writer) error {
	if len(w.Results) == 0 {
		return nil
	}
	var text strings.Builder
	for _, entry := range w.Results {
		fmt.Fprintf(&text, "==> Linting %s\n", entry.Chart)
		for _, msg := range entry.Messages {
			if entry.loadFailed {
				fmt.Fprintf(&text, "Error %s\n", msg.Err)
			} else {
				fmt.Fprintln(&text, msg)
			}
		}
		text.WriteByte('\n')
	}
	if w.ChartsFailed == 0 {
		fmt.Fprintln(&text, w.summary())
	}
	_, err := io.WriteString(out, text.String())
	return err
}

func (w *lintWriter) WriteJSON(out io.Writer) error {
	return output.EncodeJSON(out, w)
}

func (w *lintWriter) WriteYAML(out io.Writer) error {
	return output.EncodeYAML(out, w)
}
