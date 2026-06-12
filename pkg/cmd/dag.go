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
	"log/slog"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

const dagDesc = `
Print the resource sequencing DAG (directed acyclic graph) for a chart.

This is a development and troubleshooting command for HIP-0025 sequencing. It
loads the chart, evaluates conditional dependencies against the provided values,
renders templates locally, and prints the deployment order that
'helm install --wait=ordered' would use:

  - Subchart deployment batches, derived from Chart.yaml dependency 'depends-on'
    fields and the 'helm.sh/depends-on/subcharts' annotation.
  - Resource-group batches per chart level, derived from the
    'helm.sh/resource-group' and 'helm.sh/depends-on/resource-groups'
    annotations on rendered manifests.

Cycles in either DAG are reported as errors. Manifests that lack a
'helm.sh/resource-group' annotation, or whose group was demoted because of a
missing dependency, are listed as "Unsequenced" and would be deployed after the
sequenced batches.

Hooks are not part of any sequencing DAG (HIP-0025) and are omitted from the
output. No cluster connection is required; the chart is rendered with
client-side dry-run.
`

func newDagCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewInstall(cfg)
	valueOpts := &values.Options{}
	var kubeVersion string
	var extraAPIs []string

	cmd := &cobra.Command{
		Use:   "dag CHART",
		Short: "print the resource sequencing DAG for a chart",
		Long:  dagDesc,
		Args:  require.MinimumNArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compInstall(args, toComplete, client)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			if kubeVersion != "" {
				parsed, err := common.ParseKubeVersion(kubeVersion)
				if err != nil {
					return fmt.Errorf("invalid kube version %q: %w", kubeVersion, err)
				}
				client.KubeVersion = parsed
			}

			registryClient, err := newRegistryClient(out, client.CertFile, client.KeyFile, client.CaFile,
				client.InsecureSkipTLSVerify, client.PlainHTTP, client.Username, client.Password)
			if err != nil {
				return fmt.Errorf("missing registry client: %w", err)
			}
			client.SetRegistryClient(registryClient)

			// Render the chart locally without touching the cluster. Hooks are not
			// sequenced per HIP-0025, so we suppress them from the rendered output
			// to keep the DAG view focused on install-phase resources.
			client.DryRunStrategy = action.DryRunClient
			client.ReleaseName = "release-name"
			client.Replace = true
			client.APIVersions = common.VersionSet(extraAPIs)
			client.DisableHooks = true

			rel, err := runInstall(args, client, valueOpts, out)
			if err != nil {
				return err
			}
			if rel == nil || rel.Chart == nil {
				return errors.New("no chart rendered")
			}

			return printSequencingDAG(rel.Chart, strings.TrimSpace(rel.Manifest), out)
		},
	}

	f := cmd.Flags()
	addValueOptionsFlags(f, valueOpts)
	addChartPathOptionsFlags(f, &client.ChartPathOptions)
	f.StringVar(&kubeVersion, "kube-version", "", "Kubernetes version used for Capabilities.KubeVersion")
	f.StringSliceVarP(&extraAPIs, "api-versions", "a", []string{}, "Kubernetes api versions used for Capabilities.APIVersions (multiple can be specified)")
	f.BoolVar(&client.DependencyUpdate, "dependency-update", false, "update dependencies if they are missing before printing the DAG")

	return cmd
}

// printSequencingDAG walks a processed chart and its rendered manifest stream,
// printing the subchart deployment batches and per-chart resource-group batches
// in the same order 'helm install --wait=ordered' would deploy them.
func printSequencingDAG(chrt *chart.Chart, manifest string, out io.Writer) error {
	var manifests []releaseutil.Manifest
	if manifest != "" {
		parsed, err := parseTemplateManifests(manifest)
		if err != nil {
			return fmt.Errorf("parsing rendered manifests: %w", err)
		}
		manifests = parsed
	}
	return printChartLevel(chrt, manifests, chrt.Name(), 0, out)
}

func printChartLevel(chrt *chart.Chart, manifests []releaseutil.Manifest, chartPath string, depth int, out io.Writer) error {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(out, "%sChart: %s\n", indent, chartPath)

	dag, err := chartutil.BuildSubchartDAG(chrt)
	if err != nil {
		return fmt.Errorf("building subchart DAG for %s: %w", chartPath, err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		return fmt.Errorf("subchart sequencing for %s: %w", chartPath, err)
	}

	grouped := action.GroupManifestsByDirectSubchart(manifests, chartPath)

	if len(batches) == 0 {
		fmt.Fprintf(out, "%s  Subchart batches: (none)\n", indent)
	} else {
		fmt.Fprintf(out, "%s  Subchart batches:\n", indent)
		for i, batch := range batches {
			fmt.Fprintf(out, "%s    Batch %d: %s\n", indent, i+1, strings.Join(batch, ", "))
		}
	}

	if err := printResourceGroupBatches(grouped[""], indent+"  ", out); err != nil {
		return fmt.Errorf("resource-group sequencing for %s: %w", chartPath, err)
	}

	declared := make(map[string]bool)
	for _, batch := range batches {
		for _, subName := range batch {
			declared[subName] = true
			if err := printSubchartLevel(chrt, subName, grouped[subName], chartPath, depth, indent, out); err != nil {
				return err
			}
		}
	}

	// Rendered subcharts not declared in Chart.yaml dependencies (e.g. vendored
	// into charts/) are absent from the DAG but are still deployed unsequenced,
	// so the DAG view must show them too rather than silently omitting them.
	undeclared := make([]string, 0, len(grouped))
	for subName := range grouped {
		if subName == "" || declared[subName] {
			continue
		}
		undeclared = append(undeclared, subName)
	}
	sort.Strings(undeclared)
	for _, subName := range undeclared {
		fmt.Fprintf(out, "%s    Undeclared subchart %q (deployed unsequenced):\n", indent, subName)
		if err := printSubchartLevel(chrt, subName, grouped[subName], chartPath, depth, indent, out); err != nil {
			return err
		}
	}

	return nil
}

func printSubchartLevel(chrt *chart.Chart, subName string, subManifests []releaseutil.Manifest, chartPath string, depth int, indent string, out io.Writer) error {
	sub := dagFindSubchart(chrt, subName)
	if sub == nil {
		fmt.Fprintf(out, "%s    (subchart %q not found in chart dependencies)\n", indent, subName)
		return nil
	}
	subPath := chartPath + "/charts/" + subName
	return printChartLevel(sub, subManifests, subPath, depth+1, out)
}

func printResourceGroupBatches(manifests []releaseutil.Manifest, indent string, out io.Writer) error {
	if len(manifests) == 0 {
		fmt.Fprintf(out, "%sResource-group batches: (none)\n", indent)
		return nil
	}

	result, warnings, err := releaseutil.ParseResourceGroups(manifests)
	if err != nil {
		return err
	}
	for _, w := range warnings {
		slog.Warn(w)
	}

	if len(result.Groups) == 0 {
		fmt.Fprintf(out, "%sResource-group batches: (none)\n", indent)
	} else {
		groupDAG, err := releaseutil.BuildResourceGroupDAG(result)
		if err != nil {
			return err
		}
		batches, err := groupDAG.GetBatches()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%sResource-group batches:\n", indent)
		for i, batch := range batches {
			fmt.Fprintf(out, "%s  Batch %d: %s\n", indent, i+1, strings.Join(batch, ", "))
		}
	}

	if len(result.Unsequenced) > 0 {
		names := make([]string, 0, len(result.Unsequenced))
		for _, m := range result.Unsequenced {
			names = append(names, unsequencedResourceLabel(m))
		}
		sort.Strings(names)
		fmt.Fprintf(out, "%sUnsequenced (deployed last): %s\n", indent, strings.Join(names, ", "))
	}

	return nil
}

func unsequencedResourceLabel(m releaseutil.Manifest) string {
	if m.Head != nil && m.Head.Metadata != nil && m.Head.Metadata.Name != "" {
		if m.Head.Kind != "" {
			return fmt.Sprintf("%s/%s", m.Head.Kind, m.Head.Metadata.Name)
		}
		return m.Head.Metadata.Name
	}
	return m.Name
}

// dagFindSubchart mirrors the alias-aware lookup used by the action package's
// sequenced deployment so 'helm dag' resolves subcharts the same way an install
// would. Kept local to avoid widening the action package's exported surface.
func dagFindSubchart(chrt *chart.Chart, nameOrAlias string) *chart.Chart {
	aliasMap := make(map[string]string)
	if chrt.Metadata != nil {
		for _, dep := range chrt.Metadata.Dependencies {
			effective := dep.Name
			if dep.Alias != "" {
				effective = dep.Alias
			}
			aliasMap[dep.Name] = effective
		}
	}

	for _, dep := range chrt.Dependencies() {
		effective := dep.Name()
		if alias, ok := aliasMap[dep.Name()]; ok {
			effective = alias
		}
		if effective == nameOrAlias || dep.Name() == nameOrAlias {
			return dep
		}
	}
	return nil
}
