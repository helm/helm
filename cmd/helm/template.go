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
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/releaseutil"
)

const templateDesc = `
Render chart templates locally and display the output.

Any values that would normally be looked up or retrieved in-cluster will be
faked locally. Additionally, none of the server-side testing of chart validity
(e.g. whether an API is supported) is done.
`

func newTemplateCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var validate bool
	var includeCrds bool
	client := action.NewInstall(cfg)
	valueOpts := &values.Options{}
	var extraAPIs []string
	var showFiles []string

	cmd := &cobra.Command{
		Use:   "template [NAME] [CHART]",
		Short: "locally render templates",
		Long:  templateDesc,
		Args:  require.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client.DryRun = true
			client.ReleaseName = "RELEASE-NAME"
			client.Replace = true // Skip the name check
			client.ClientOnly = !validate
			client.APIVersions = chartutil.VersionSet(extraAPIs)
			client.IncludeCRDs = includeCrds
			rel, err := runInstall(args, client, valueOpts, out)

			if err != nil && !settings.Debug {
				if rel != nil {
					return fmt.Errorf("%w\n\nUse --debug flag to render out invalid YAML", err)
				}
				return err
			}

			// We ignore a potential error here because, when the --debug flag was specified,
			// we always want to print the YAML, even if it is not valid. The error is still returned afterwards.
			if rel != nil {
				var manifests bytes.Buffer
				fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))

				if !client.DisableHooks {
					for _, m := range rel.Hooks {
						fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
					}
				}

				// if we have a list of files to render, then check that each of the
				// provided files exists in the chart.
				if len(showFiles) > 0 {
					// This is necessary to ensure consistent manifest ordering when using --show-only
					// with globs or directory names.
					splitManifests := releaseutil.SplitManifests(manifests.String())
					manifestsKeys := make([]string, 0, len(splitManifests))
					for k := range splitManifests {
						manifestsKeys = append(manifestsKeys, k)
					}
					sort.Sort(releaseutil.BySplitManifestsOrder(manifestsKeys))

					manifestNameRegex := regexp.MustCompile("# Source: [^/]+/(.+)")
					var manifestsToRender []string
					for _, f := range showFiles {
						missing := true
						for _, manifestKey := range manifestsKeys {
							manifest := splitManifests[manifestKey]
							submatch := manifestNameRegex.FindStringSubmatch(manifest)
							if len(submatch) == 0 {
								continue
							}
							manifestName := submatch[1]
							// manifest.Name is rendered using linux-style filepath separators on Windows as
							// well as macOS/linux.
							manifestPathSplit := strings.Split(manifestName, "/")
							manifestPath := filepath.Join(manifestPathSplit...)

							// if the filepath provided matches a manifest path in the
							// chart, render that manifest
							if matched, _ := filepath.Match(f, manifestPath); !matched {
								continue
							}
							manifestsToRender = append(manifestsToRender, manifest)
							missing = false
						}
						if missing {
							return fmt.Errorf("could not find template %s in chart", f)
						}
					}
					for _, m := range manifestsToRender {
						fmt.Fprintf(out, "---\n%s\n", m)
					}
				} else {
					fmt.Fprintf(out, "%s", manifests.String())
				}
			}

			return err
		},
	}

	// Function providing dynamic auto-completion
	completion.RegisterValidArgsFunc(cmd, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		return compInstall(args, toComplete, client)
	})

	f := cmd.Flags()
	addInstallFlags(f, client, valueOpts)
	f.StringArrayVarP(&showFiles, "show-only", "s", []string{}, "only show manifests rendered from the given templates")
	f.StringVar(&client.OutputDir, "output-dir", "", "writes the executed templates to files in output-dir instead of stdout")
	f.BoolVar(&validate, "validate", false, "validate your manifests against the Kubernetes cluster you are currently pointing at. This is the same validation performed on an install")
	f.BoolVar(&includeCrds, "include-crds", false, "include CRDs in the templated output")
	f.BoolVar(&client.IsUpgrade, "is-upgrade", false, "set .Release.IsUpgrade instead of .Release.IsInstall")
	f.StringArrayVarP(&extraAPIs, "api-versions", "a", []string{}, "Kubernetes api versions used for Capabilities.APIVersions")
	f.BoolVar(&client.UseReleaseName, "release-name", false, "use release name in the output-dir path.")
	bindPostRenderFlag(cmd, &client.PostRenderer)

	return cmd
}
