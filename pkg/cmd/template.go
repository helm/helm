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
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	release "helm.sh/helm/v4/pkg/release/v1"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release/v1/sequence"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

const templateDesc = `
Render chart templates locally and display the output.

Any values that would normally be looked up or retrieved in-cluster will be
faked locally. Additionally, none of the server-side testing of chart validity
(e.g. whether an API is supported) is done.

To specify the Kubernetes API versions used for Capabilities.APIVersions, use
the '--api-versions' flag. This flag can be specified multiple times or as a
comma-separated list:

    $ helm template --api-versions networking.k8s.io/v1 --api-versions cert-manager.io/v1 mychart ./mychart

or

    $ helm template --api-versions networking.k8s.io/v1,cert-manager.io/v1 mychart ./mychart
`

func newTemplateCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	var validate bool
	var includeCrds bool
	var skipTests bool
	client := action.NewInstall(cfg)
	valueOpts := &values.Options{}
	var kubeVersion string
	var extraAPIs []string
	var showFiles []string

	cmd := &cobra.Command{
		Use:   "template [NAME] [CHART]",
		Short: "locally render templates",
		Long:  templateDesc,
		Args:  require.MinimumNArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compInstall(args, toComplete, client)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if kubeVersion != "" {
				parsedKubeVersion, err := common.ParseKubeVersion(kubeVersion)
				if err != nil {
					return fmt.Errorf("invalid kube version '%s': %w", kubeVersion, err)
				}
				client.KubeVersion = parsedKubeVersion
			}

			registryClient, err := newRegistryClient(out, client.CertFile, client.KeyFile, client.CaFile,
				client.InsecureSkipTLSVerify, client.PlainHTTP, client.Username, client.Password)
			if err != nil {
				return fmt.Errorf("missing registry client: %w", err)
			}
			client.SetRegistryClient(registryClient)

			dryRunStrategy, err := cmdGetDryRunFlagStrategy(cmd, true)
			if err != nil {
				return err
			}
			if validate {
				// Mimic deprecated --validate flag behavior by enabling server dry run
				dryRunStrategy = action.DryRunServer
			}
			client.DryRunStrategy = dryRunStrategy
			client.ReleaseName = "release-name"
			client.Replace = true // Skip the name check
			client.APIVersions = common.VersionSet(extraAPIs)
			client.IncludeCRDs = includeCrds
			orderedTemplateOutput := client.WaitStrategy == kube.OrderedWaitStrategy && len(showFiles) == 0 && client.OutputDir == ""
			rel, err := runInstall(args, client, valueOpts, out)

			if err != nil && !settings.Debug {
				if rel != nil {
					return fmt.Errorf("%w\n\nUse --debug flag to render out invalid YAML", err)
				}
				return err
			}
			installErr := err

			// We ignore a potential error here because, when the --debug flag was specified,
			// we always want to print the YAML, even if it is not valid. The error is still returned afterwards.
			if rel != nil {
				orderedRendered := false
				if orderedTemplateOutput {
					if renderErr := renderOrderedTemplate(rel.Chart, strings.TrimSpace(rel.Manifest), out); renderErr != nil {
						// Honor the --debug contract: always print the manifests, even if
						// ordered rendering fails (e.g., a document fails YAML structural
						// parsing). Fall back to the flat path with a stderr warning.
						fmt.Fprintf(os.Stderr, "WARNING: ordered template rendering failed (%v); falling back to flat output\n", renderErr)
					} else {
						orderedRendered = true
						if !client.DisableHooks {
							for _, m := range rel.Hooks {
								if skipTests && isTestHook(m) {
									continue
								}
								fmt.Fprintf(out, "---\n# Source: %s\n%s\n", m.Path, releaseutil.StripHelmInternalAnnotations(m.Manifest))
							}
						}
					}
				}
				if !orderedRendered {
					var manifests bytes.Buffer
					fmt.Fprintln(&manifests, strings.TrimSpace(releaseutil.StripHelmInternalAnnotations(rel.Manifest)))
					if !client.DisableHooks {
						fileWritten := make(map[string]bool)
						for _, m := range rel.Hooks {
							if skipTests && isTestHook(m) {
								continue
							}
							if client.OutputDir == "" {
								fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, releaseutil.StripHelmInternalAnnotations(m.Manifest))
							} else {
								newDir := client.OutputDir
								if client.UseReleaseName {
									newDir = filepath.Join(client.OutputDir, client.ReleaseName)
								}
								_, err := os.Stat(filepath.Join(newDir, m.Path))
								if err == nil {
									fileWritten[m.Path] = true
								}

								err = writeToFile(newDir, m.Path, releaseutil.StripHelmInternalAnnotations(m.Manifest), fileWritten[m.Path])
								if err != nil {
									return err
								}
							}
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
							// Use linux-style filepath separators to unify user's input path
							f = filepath.ToSlash(f)
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
								// manifest.Path is connected using linux-style filepath separators on Windows as
								// well as macOS/linux
								manifestPath := strings.Join(manifestPathSplit, "/")

								// if the filepath provided matches a manifest path in the
								// chart, render that manifest
								if matched, _ := filepath.Match(f, manifestPath); !matched {
									continue
								}
								manifestsToRender = append(manifestsToRender, manifest)
								missing = false
							}
							if missing {
								if installErr != nil && settings.Debug {
									// assume the manifest itself is too malformed to be rendered
									return installErr
								}
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
			}

			return installErr
		},
	}

	f := cmd.Flags()
	addInstallFlags(cmd, f, client, valueOpts)
	f.StringArrayVarP(&showFiles, "show-only", "s", []string{}, "only show manifests rendered from the given templates")
	f.StringVar(&client.OutputDir, "output-dir", "", "writes the executed templates to files in output-dir instead of stdout")
	f.BoolVar(&validate, "validate", false, "deprecated")
	f.MarkDeprecated("validate", "use '--dry-run=server' instead")
	f.BoolVar(&includeCrds, "include-crds", false, "include CRDs in the templated output")
	f.BoolVar(&skipTests, "skip-tests", false, "skip tests from templated output")
	f.BoolVar(&client.IsUpgrade, "is-upgrade", false, "set .Release.IsUpgrade instead of .Release.IsInstall")
	f.StringVar(&kubeVersion, "kube-version", "", "Kubernetes version used for Capabilities.KubeVersion")
	f.StringSliceVarP(&extraAPIs, "api-versions", "a", []string{}, "Kubernetes api versions used for Capabilities.APIVersions (multiple can be specified)")
	f.BoolVar(&client.UseReleaseName, "release-name", false, "use release name in the output-dir path.")
	f.String(
		"dry-run",
		"client",
		`simulates the operation either client-side or server-side. Must be either: "client", or "server". '--dry-run=client simulates the operation client-side only and avoids cluster connections. '--dry-run=server' simulates/validates the operation on the server, requiring cluster connectivity.`)
	f.Lookup("dry-run").NoOptDefVal = "unset"
	bindPostRenderFlag(cmd, &client.PostRenderer, settings)
	cmd.MarkFlagsMutuallyExclusive("validate", "dry-run")

	return cmd
}

func renderOrderedTemplate(chrt *chart.Chart, manifest string, out io.Writer) error {
	if manifest == "" {
		return nil
	}

	manifests, err := sequence.ParseStoredManifests(manifest)
	if err != nil {
		// Return the parse error so the caller falls back to the flat-output
		// path, which strips Helm-internal annotations before emitting. Writing
		// the raw manifest here would re-emit stripped sequencing annotations
		// (e.g. helm.sh/depends-on/resource-groups) and break the invariant that
		// `helm template` output stays directly apply-able. No output has been
		// written to `out` yet at this point, so the fallback cannot duplicate.
		return err
	}
	plan, err := sequence.Build(chrt, manifests)
	if err != nil {
		// Return the plan error so the caller falls back to the flat-output
		// path, preserving `helm template`'s apply-ready annotation stripping
		// contract while still surfacing cycles or invalid multi-group resources.
		return err
	}
	logSequencePlanWarnings(plan)

	// Render into a buffer so we can normalize trailing whitespace to match
	// the flat path, which TrimSpaces the whole manifest blob then writes a
	// single trailing newline (template.go flat branch). Per-manifest emission
	// would otherwise leave one trailing blank line after the final document,
	// breaking the HIP-0025 byte-for-byte backwards-compat guarantee for charts
	// with no sequencing annotations.
	var buf bytes.Buffer
	for _, batch := range plan.Batches {
		switch batch.Kind {
		case sequence.BatchKindGroups:
			for _, group := range batch.Groups {
				fmt.Fprintf(&buf, "## START resource-group: %s %s\n", sequence.DisplayPath(batch.ChartPath), group.Name)
				for _, manifest := range group.Manifests {
					writeOrderedManifest(&buf, manifest.Content)
				}
				fmt.Fprintf(&buf, "## END resource-group: %s %s\n", sequence.DisplayPath(batch.ChartPath), group.Name)
			}
		case sequence.BatchKindUnsequenced:
			for _, manifest := range batch.Manifests() {
				writeOrderedManifest(&buf, manifest.Content)
			}
		}
	}
	_, err = fmt.Fprintln(out, strings.TrimRight(buf.String(), "\n"))
	return err
}

// logSequencePlanWarnings surfaces non-fatal sequencing-plan warnings with the
// same shape as the action layer's logPlanWarnings.
func logSequencePlanWarnings(plan *sequence.Plan) {
	for _, w := range plan.Warnings {
		slog.Warn("sequencing: "+w.Message, "chart", w.ChartPath)
	}
}

// writeOrderedManifest emits a single manifest document with the same
// inter-document whitespace as the flat `helm template` path
// (pkg/action/action.go), which writes "---\n# Source: %s\n%s\n" per
// manifest using renderer-supplied content that always ends with a newline.
// SplitManifests strips trailing newlines from intermediate chunks, so we
// normalize to exactly one trailing newline here before adding the format's
// own trailing "\n" — producing "---\nCONTENT\n\n" between docs and keeping
// `helm template` byte-identical to default mode for charts with no
// sequencing annotations (HIP-0025 backwards-compat guarantee, S04-05).
func writeOrderedManifest(out io.Writer, content string) {
	stripped := releaseutil.StripHelmInternalAnnotations(content)
	stripped = strings.TrimRight(stripped, "\n") + "\n"
	fmt.Fprintf(out, "---\n%s\n", stripped)
}

func isTestHook(h *release.Hook) bool {
	return slices.Contains(h.Events, release.HookTest)
}

// The following functions (writeToFile, createOrOpenFile, and ensureDirectoryForFile)
// are copied from the actions package. This is part of a change to correct a
// bug introduced by #8156. As part of the todo to refactor renderResources
// this duplicate code should be removed. It is added here so that the API
// surface area is as minimally impacted as possible in fixing the issue.
func writeToFile(outputDir string, name string, data string, appendData bool) error {
	outfileName := outputDir + string(filepath.Separator) + name

	err := ensureDirectoryForFile(outfileName)
	if err != nil {
		return err
	}

	f, err := createOrOpenFile(outfileName, appendData)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = fmt.Fprintf(f, "---\n# Source: %s\n%s\n", name, data)

	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", outfileName)
	return nil
}

func createOrOpenFile(filename string, appendData bool) (*os.File, error) {
	if appendData {
		return os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0o600)
	}
	return os.Create(filename)
}

func ensureDirectoryForFile(file string) error {
	baseDir := filepath.Dir(file)
	_, err := os.Stat(baseDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return os.MkdirAll(baseDir, 0o755)
}
