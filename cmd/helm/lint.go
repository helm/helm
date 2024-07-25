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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/lint/support"
	"helm.sh/helm/v3/pkg/lint/rules"
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
	var lintIgnoreFile string

	cmd := &cobra.Command{
		Use:   "lint PATH",
		Short: "examine a chart for possible issues",
		Long:  longLintHelp,
		RunE: func(_ *cobra.Command, args []string) error {
			paths := []string{"."}
			if len(args) > 0 {
				paths = args
			}
			if kubeVersion != "" {
				parsedKubeVersion, err := chartutil.ParseKubeVersion(kubeVersion)
				if err != nil {
					return fmt.Errorf("invalid kube version '%s': %s", kubeVersion, err)
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
			var ignorePatterns map[string][]string
			if lintIgnoreFile != "" {
				fmt.Printf("\nUsing ignore file: %s\n", lintIgnoreFile)
				ignorePatterns, err = rules.ParseIgnoreFile(lintIgnoreFile)
				// if err != nil {
				// 	return fmt.Errorf("failed to parse .helmlintignore file: %v", err)
				// }
			}
			var message strings.Builder
			failed := 0
			for _, path := range paths {
				result := client.Run([]string{path}, vals)
				filteredResult := FilterIgnoredMessages(result, ignorePatterns)
				fmt.Fprintf(&message, "==> Linting %s\n", path)
				for _, msg := range filteredResult.Messages {
					fmt.Fprintf(&message, "%s\n", msg)
				}
				if len(filteredResult.Messages) != 0 {
					failed++
					for _, err := range filteredResult.Errors {
						fmt.Fprintf(&message, "Error: %s\n", err)
					}
				}
			}

			fmt.Fprint(out, message.String())
			summary := fmt.Sprintf("%d chart(s) linted, %d chart(s) failed", len(paths), failed)
			if failed > 0 {
				return errors.New(summary)
			}
			fmt.Fprintln(out, summary)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&client.Strict, "strict", false, "fail on lint warnings")
	f.BoolVar(&client.WithSubcharts, "with-subcharts", false, "lint dependent charts")
	f.BoolVar(&client.Quiet, "quiet", false, "print only warnings and errors")
	f.StringVar(&kubeVersion, "kube-version", "", "Kubernetes version used for capabilities and deprecation checks")
	f.StringVar(&lintIgnoreFile, "lint-ignore-file", "", "path to .helmlintignore file to specify ignore patterns")

	return cmd
}

func FilterIgnoredMessages(result *action.LintResult, patterns map[string][]string) *action.LintResult {
    filteredMessages := make([]support.Message, 0)
    for _, msg := range result.Messages {
        fullPath := extractFullPathFromError(msg.Err.Error())
		if len(fullPath) == 0 {
			break
		}
        fmt.Printf("Extracted full path: %s\n", fullPath)
        ignore := false
        for path, pathPatterns := range patterns {
            if strings.Contains(fullPath, filepath.Clean(path)) {
                for _, pattern := range pathPatterns {
                    if strings.Contains(msg.Err.Error(), pattern) {
                        fmt.Printf("Ignoring message: [%s] %s\n\n", fullPath, msg.Err.Error())
                        ignore = true
                        break
                    }
                }
            }
            if ignore {
                break
            }
        }
        if !ignore {
            filteredMessages = append(filteredMessages, msg)
        }
    }
    return &action.LintResult{Messages: filteredMessages}
}

func extractFullPathFromError(errorString string) string {
    parts := strings.Split(errorString, ":")
    if len(parts) > 2 {
        return strings.TrimSpace(parts[1])
    }
    return ""
}
