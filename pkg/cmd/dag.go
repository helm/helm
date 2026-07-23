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
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/release/v1/sequence"
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
		parsed, err := sequence.ParseStoredManifests(manifest)
		if err != nil {
			return fmt.Errorf("parsing rendered manifests: %w", err)
		}
		manifests = parsed
	}
	plan, err := sequence.Build(chrt, manifests)
	if err != nil {
		return err
	}
	logSequencePlanWarnings(plan)

	levelByPath := make(map[string]*sequence.ChartLevel, len(plan.Levels))
	for i := range plan.Levels {
		levelByPath[plan.Levels[i].Path] = &plan.Levels[i]
	}

	groupBatchesByPath := make(map[string][]sequence.Batch)
	unsequencedByPath := make(map[string]sequence.Batch)
	for _, batch := range plan.Batches {
		switch batch.Kind {
		case sequence.BatchKindGroups:
			groupBatchesByPath[batch.ChartPath] = append(groupBatchesByPath[batch.ChartPath], batch)
		case sequence.BatchKindUnsequenced:
			unsequencedByPath[batch.ChartPath] = batch
		}
	}

	var printLevel func(level *sequence.ChartLevel)
	printLevel = func(level *sequence.ChartLevel) {
		indent := strings.Repeat("  ", level.Depth)
		fmt.Fprintf(out, "%sChart: %s\n", indent, level.Path)

		if len(level.SubchartBatches) == 0 {
			fmt.Fprintf(out, "%s  Subchart batches: (none)\n", indent)
		} else {
			fmt.Fprintf(out, "%s  Subchart batches:\n", indent)
			for i, batch := range level.SubchartBatches {
				fmt.Fprintf(out, "%s    Batch %d: %s\n", indent, i+1, strings.Join(batch, ", "))
			}
		}

		resourceIndent := indent + "  "
		groupBatches := groupBatchesByPath[level.Path]
		if len(groupBatches) == 0 {
			fmt.Fprintf(out, "%sResource-group batches: (none)\n", resourceIndent)
		} else {
			fmt.Fprintf(out, "%sResource-group batches:\n", resourceIndent)
			for i, batch := range groupBatches {
				names := make([]string, 0, len(batch.Groups))
				for _, group := range batch.Groups {
					names = append(names, group.Name)
				}
				fmt.Fprintf(out, "%s  Batch %d: %s\n", resourceIndent, i+1, strings.Join(names, ", "))
			}
		}

		if batch, ok := unsequencedByPath[level.Path]; ok {
			names := make([]string, 0, len(batch.Manifests()))
			for _, manifest := range batch.Manifests() {
				names = append(names, unsequencedResourceLabel(manifest))
			}
			sort.Strings(names)
			fmt.Fprintf(out, "%sUnsequenced (deployed last): %s\n", resourceIndent, strings.Join(names, ", "))
		}

		printChild := func(name string) {
			if slices.Contains(level.Unresolved, name) {
				fmt.Fprintf(out, "%s    (subchart %q metadata unavailable; sequenced structurally from manifests)\n", indent, name)
			}
			if child := levelByPath[level.Path+"/charts/"+name]; child != nil {
				printLevel(child)
			}
		}

		for _, batch := range level.SubchartBatches {
			for _, name := range batch {
				printChild(name)
			}
		}
		for _, name := range level.Undeclared {
			fmt.Fprintf(out, "%s    Undeclared subchart %q (deployed unsequenced):\n", indent, name)
			printChild(name)
		}
	}

	if len(plan.Levels) > 0 {
		printLevel(&plan.Levels[0])
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
