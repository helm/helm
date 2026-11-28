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
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/internal/chart/v3/loader"
	chartutil "helm.sh/helm/v4/internal/chart/v3/util"
	"helm.sh/helm/v4/internal/gates"
	release "helm.sh/helm/v4/internal/release/v2"
	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
	"helm.sh/helm/v4/internal/release/v2/sequence"
	"helm.sh/helm/v4/pkg/chart/common"
	commonutil "helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/engine"
	"helm.sh/helm/v4/pkg/getter"
)

const dagDesc = `Print the subchart sequencing DAG for a chart API version v3 chart. The chart is rendered locally without a cluster; hooks are omitted.`

func newDagCmd(out io.Writer) *cobra.Command {
	valueOpts := &values.Options{}
	var kubeVersion string
	var extraAPIs []string
	cmd := &cobra.Command{
		Use:   "dag CHART",
		Short: "print the subchart sequencing DAG for a chart",
		Long:  dagDesc,
		Args:  require.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !gates.ChartV3.IsEnabled() {
				return gates.ChartV3.Error()
			}
			chrt, err := loader.Load(args[0])
			if err != nil {
				return err
			}
			if chrt.Metadata.APIVersion != chart.APIVersionV3 {
				return fmt.Errorf("helm dag requires chart apiVersion v3 (chart %q has apiVersion %s)", chrt.Name(), chrt.Metadata.APIVersion)
			}
			vals, err := valueOpts.MergeValues(getter.All(settings))
			if err != nil {
				return err
			}
			if err := chartutil.ProcessDependencies(chrt, vals); err != nil {
				return err
			}
			caps := common.DefaultCapabilities.Copy()
			if kubeVersion != "" {
				parsed, err := common.ParseKubeVersion(kubeVersion)
				if err != nil {
					return fmt.Errorf("invalid kube version %q: %w", kubeVersion, err)
				}
				caps.KubeVersion = *parsed
			}
			caps.APIVersions = append(slices.Clone(caps.APIVersions), extraAPIs...)
			coalesced, err := commonutil.CoalesceValues(chrt, vals)
			if err != nil {
				return err
			}
			renderValues, err := commonutil.ToRenderValues(chrt, coalesced, common.ReleaseOptions{
				Name: "release-name", Namespace: settings.Namespace(), Revision: 1, IsInstall: true,
			}, caps)
			if err != nil {
				return err
			}
			rendered, err := (engine.Engine{}).RenderWithContext(context.Background(), chrt, renderValues)
			if err != nil {
				return err
			}
			plan, err := sequence.Build(chrt, dagManifests(rendered))
			if err != nil {
				return err
			}
			for _, warning := range plan.Warnings {
				slog.Warn("sequencing: "+warning.Message, "chart", warning.ChartPath)
			}
			printSequencingDAG(plan, out)
			return nil
		},
	}
	addValueOptionsFlags(cmd.Flags(), valueOpts)
	cmd.Flags().StringVar(&kubeVersion, "kube-version", "", "Kubernetes version used for Capabilities.KubeVersion")
	cmd.Flags().StringSliceVarP(&extraAPIs, "api-versions", "a", nil, "Kubernetes API versions used for Capabilities.APIVersions")
	return cmd
}

func dagManifests(rendered map[string]string) []releaseutil.Manifest {
	var manifests []releaseutil.Manifest
	for _, path := range slices.Sorted(maps.Keys(rendered)) {
		documents := releaseutil.SplitManifests(rendered[path])
		for _, name := range slices.Sorted(maps.Keys(documents)) {
			var head releaseutil.SimpleHead
			if yaml.Unmarshal([]byte(documents[name]), &head) != nil {
				continue
			}
			manifest := releaseutil.Manifest{Name: path, Content: documents[name], Head: &head}
			if head.Metadata == nil || strings.TrimSpace(head.Metadata.Annotations[release.HookAnnotation]) == "" {
				manifests = append(manifests, manifest)
			}
		}
	}
	return manifests
}

func printSequencingDAG(plan *sequence.Plan, out io.Writer) {
	levels := make(map[string]*sequence.ChartLevel, len(plan.Levels))
	manifests := make(map[string][]releaseutil.Manifest)
	for i := range plan.Levels {
		levels[plan.Levels[i].Path] = &plan.Levels[i]
	}
	for _, batch := range plan.Batches {
		manifests[batch.ChartPath] = append(manifests[batch.ChartPath], batch.Manifests...)
	}
	var printLevel func(*sequence.ChartLevel)
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
		labels := make([]string, 0, len(manifests[level.Path]))
		for _, manifest := range manifests[level.Path] {
			label := manifest.Name
			if manifest.Head != nil && manifest.Head.Metadata != nil && manifest.Head.Metadata.Name != "" {
				label = manifest.Head.Metadata.Name
				if manifest.Head.Kind != "" {
					label = manifest.Head.Kind + "/" + label
				}
			}
			labels = append(labels, label)
		}
		if len(labels) > 0 {
			sort.Strings(labels)
			fmt.Fprintf(out, "%s  Unsequenced (deployed last): %s\n", indent, strings.Join(labels, ", "))
		}
		printChild := func(name string) {
			if slices.Contains(level.Unresolved, name) {
				fmt.Fprintf(out, "%s    (subchart %q metadata unavailable; sequenced structurally from manifests)\n", indent, name)
			}
			if child := levels[level.Path+"/charts/"+name]; child != nil {
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
}
