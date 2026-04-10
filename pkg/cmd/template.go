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
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	release "helm.sh/helm/v4/pkg/release/v1"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/kube"
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

			registryClient, err := newRegistryClient(client.CertFile, client.KeyFile, client.CaFile,
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

			// We ignore a potential error here because, when the --debug flag was specified,
			// we always want to print the YAML, even if it is not valid. The error is still returned afterwards.
			if rel != nil {
				if orderedTemplateOutput {
					templateChart, err := loadTemplateChart(args, client)
					if err != nil {
						return err
					}
					if err := renderOrderedTemplate(templateChart, strings.TrimSpace(rel.Manifest), out); err != nil {
						return err
					}
					if !client.DisableHooks {
						for _, m := range rel.Hooks {
							if skipTests && isTestHook(m) {
								continue
							}
							fmt.Fprintf(out, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
						}
					}
				} else {
					var manifests bytes.Buffer
					fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
					if !client.DisableHooks {
						fileWritten := make(map[string]bool)
						for _, m := range rel.Hooks {
							if skipTests && isTestHook(m) {
								continue
							}
							if client.OutputDir == "" {
								fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", m.Path, m.Manifest)
							} else {
								newDir := client.OutputDir
								if client.UseReleaseName {
									newDir = filepath.Join(client.OutputDir, client.ReleaseName)
								}
								_, err := os.Stat(filepath.Join(newDir, m.Path))
								if err == nil {
									fileWritten[m.Path] = true
								}

								err = writeToFile(newDir, m.Path, m.Manifest, fileWritten[m.Path])
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

			return err
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

func loadTemplateChart(args []string, client *action.Install) (*chart.Chart, error) {
	_, chartRef, err := client.NameAndChart(args)
	if err != nil {
		return nil, err
	}

	chartPath, err := client.LocateChart(chartRef, settings)
	if err != nil {
		return nil, err
	}

	return loader.Load(chartPath)
}

func renderOrderedTemplate(chrt *chart.Chart, manifest string, out io.Writer) error {
	if manifest == "" {
		return nil
	}

	sortedManifests, err := parseTemplateManifests(manifest)
	if err != nil {
		return fmt.Errorf("parsing manifests for ordered output: %w", err)
	}

	return renderOrderedChartLevel(chrt, sortedManifests, chrt.Name(), out)
}

func parseTemplateManifests(manifest string) ([]releaseutil.Manifest, error) {
	rawManifests := releaseutil.SplitManifests(manifest)
	keys := make([]string, 0, len(rawManifests))
	for key := range rawManifests {
		keys = append(keys, key)
	}
	sort.Sort(releaseutil.BySplitManifestsOrder(keys))

	manifests := make([]releaseutil.Manifest, 0, len(keys))
	for _, key := range keys {
		content := rawManifests[key]
		name := manifestSourcePath(content)
		if name == "" {
			name = key
		}

		var head releaseutil.SimpleHead
		if err := yaml.Unmarshal([]byte(content), &head); err != nil {
			return nil, fmt.Errorf("YAML parse error on %s: %w", name, err)
		}

		manifests = append(manifests, releaseutil.Manifest{
			Name:    name,
			Content: content,
			Head:    &head,
		})
	}

	return manifests, nil
}

func manifestSourcePath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# Source: ") {
			return strings.TrimPrefix(line, "# Source: ")
		}
	}

	return ""
}

func renderOrderedChartLevel(chrt *chart.Chart, manifests []releaseutil.Manifest, chartPath string, out io.Writer) error {
	if len(manifests) == 0 {
		return nil
	}

	grouped := groupManifestsByChartPath(manifests, chartPath)
	renderedSubcharts := make(map[string]struct{})

	dag, err := chartutil.BuildSubchartDAG(chrt)
	if err != nil {
		return fmt.Errorf("building subchart DAG for %s: %w", chartPath, err)
	}

	batches, err := dag.GetBatches()
	if err != nil {
		return fmt.Errorf("getting subchart batches for %s: %w", chartPath, err)
	}

	for _, batch := range batches {
		for _, subchartName := range batch {
			if err := renderOrderedSubchart(chrt, chartPath, subchartName, grouped[subchartName], out); err != nil {
				return err
			}
			renderedSubcharts[subchartName] = struct{}{}
		}
	}

	var remainingSubcharts []string
	for subchartName := range grouped {
		if subchartName == "" {
			continue
		}
		if _, ok := renderedSubcharts[subchartName]; ok {
			continue
		}
		remainingSubcharts = append(remainingSubcharts, subchartName)
	}
	sort.Strings(remainingSubcharts)

	for _, subchartName := range remainingSubcharts {
		if err := renderOrderedSubchart(chrt, chartPath, subchartName, grouped[subchartName], out); err != nil {
			return err
		}
	}

	return renderOrderedResourceGroups(grouped[""], chartPath, out)
}

func renderOrderedSubchart(chrt *chart.Chart, chartPath, subchartName string, manifests []releaseutil.Manifest, out io.Writer) error {
	if len(manifests) == 0 {
		return nil
	}

	subchartPath := path.Join(chartPath, subchartName)
	subChart := findTemplateSubchart(chrt, subchartName)
	if subChart == nil {
		slog.Warn("subchart not found in chart dependencies during template rendering; falling back to flat ordered output", "subchart", subchartName)
		return renderOrderedResourceGroups(manifests, subchartPath, out)
	}

	return renderOrderedChartLevel(subChart, manifests, subchartPath, out)
}

func renderOrderedResourceGroups(manifests []releaseutil.Manifest, chartPath string, out io.Writer) error {
	if len(manifests) == 0 {
		return nil
	}

	result, warnings, err := releaseutil.ParseResourceGroups(manifests)
	if err != nil {
		return fmt.Errorf("parsing resource groups for %s: %w", chartPath, err)
	}
	for _, warning := range warnings {
		slog.Warn("resource-group annotation warning during template rendering", "chartPath", chartPath, "warning", warning)
	}

	if len(result.Groups) > 0 {
		dag, err := releaseutil.BuildResourceGroupDAG(result)
		if err != nil {
			return fmt.Errorf("building resource-group DAG for %s: %w", chartPath, err)
		}
		batches, err := dag.GetBatches()
		if err != nil {
			return fmt.Errorf("getting resource-group batches for %s: %w", chartPath, err)
		}

		for _, batch := range batches {
			for _, groupName := range batch {
				fmt.Fprintf(out, "## START resource-group: %s %s\n", chartPath, groupName)
				for _, manifest := range result.Groups[groupName] {
					fmt.Fprintf(out, "---\n%s\n", manifest.Content)
				}
				fmt.Fprintf(out, "## END resource-group: %s %s\n", chartPath, groupName)
			}
		}
	}

	for _, manifest := range result.Unsequenced {
		fmt.Fprintf(out, "---\n%s\n", manifest.Content)
	}

	return nil
}

func groupManifestsByChartPath(manifests []releaseutil.Manifest, chartPath string) map[string][]releaseutil.Manifest {
	result := make(map[string][]releaseutil.Manifest)
	chartsPrefix := chartManifestPrefix(chartPath) + "/charts/"

	for _, manifest := range manifests {
		if !strings.HasPrefix(manifest.Name, chartsPrefix) {
			result[""] = append(result[""], manifest)
			continue
		}

		rest := manifest.Name[len(chartsPrefix):]
		idx := strings.Index(rest, "/")
		if idx < 0 {
			result[""] = append(result[""], manifest)
			continue
		}

		subchartName := rest[:idx]
		result[subchartName] = append(result[subchartName], manifest)
	}

	return result
}

func chartManifestPrefix(chartPath string) string {
	parts := strings.Split(chartPath, "/")
	if len(parts) == 0 {
		return chartPath
	}

	prefix := parts[0]
	for _, part := range parts[1:] {
		prefix = path.Join(prefix, "charts", part)
	}

	return prefix
}

func findTemplateSubchart(chrt *chart.Chart, nameOrAlias string) *chart.Chart {
	if chrt == nil || chrt.Metadata == nil {
		return nil
	}

	aliases := make(map[string]string, len(chrt.Metadata.Dependencies))
	for _, dep := range chrt.Metadata.Dependencies {
		effectiveName := dep.Name
		if dep.Alias != "" {
			effectiveName = dep.Alias
		}
		aliases[dep.Name] = effectiveName
	}

	for _, dep := range chrt.Dependencies() {
		effectiveName := dep.Name()
		if alias, ok := aliases[dep.Name()]; ok {
			effectiveName = alias
		}
		if effectiveName == nameOrAlias || dep.Name() == nameOrAlias {
			return dep
		}
	}

	return nil
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
	outfileName := strings.Join([]string{outputDir, name}, string(filepath.Separator))

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
		return os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	}
	return os.Create(filename)
}

func ensureDirectoryForFile(file string) error {
	baseDir := filepath.Dir(file)
	_, err := os.Stat(baseDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return os.MkdirAll(baseDir, 0755)
}
