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

// Package sequence computes the ordered deployment plan for a release's
// rendered manifests per HIP-0025: a two-level walk over the chart's subchart
// DAG and each chart level's resource-group DAG, flattened into an ordered
// list of apply-and-wait batches. The package is pure — it never talks to a
// cluster and must not import pkg/kube, pkg/action, or pkg/cmd — so the same
// plan is usable by install/upgrade drivers, uninstall/rollback (reversed),
// `helm template`/`helm dag` rendering, and lint.
package sequence

import (
	"strings"

	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

// BatchKind distinguishes the two batch flavors a chart level produces.
type BatchKind uint8

const (
	// BatchKindGroups: one topological level of the resource-group DAG —
	// one or more groups deployable in parallel.
	BatchKindGroups BatchKind = iota
	// BatchKindUnsequenced: a chart level's trailing batch — resources with no
	// group annotation, resources demoted for invalid depends-on JSON,
	// groups demoted for missing references, and isolated groups.
	BatchKindUnsequenced
)

// Group is a named set of manifests deployed as one unit within a batch.
type Group struct {
	Name      string                 // "" only inside a BatchKindUnsequenced batch
	Manifests []releaseutil.Manifest // input order preserved
}

// Batch is one apply-and-wait unit. Drivers process Batches strictly in order.
type Batch struct {
	// ChartPath is the manifest path-prefix of the owning chart level:
	// "parent", "parent/charts/db", "parent/charts/db/charts/redis".
	// "" when the plan was built without a chart (flat fallback).
	ChartPath string
	// Depth is the chart-nesting depth (0 = top level). Display-only.
	Depth int
	Kind  BatchKind
	// Groups: for BatchKindGroups, ≥1 groups sorted by Name (one DAG level);
	// for BatchKindUnsequenced, exactly one Group{Name: ""}.
	Groups []Group
	// Wait: gate on readiness (install/upgrade) or deletion (reversed plans)
	// before starting the next batch. The builder sets true for every batch;
	// the field exists so a strict-HIP mode can flip leaf-only batches later.
	Wait bool
	// HasCustomReadiness: at least one manifest in the batch carries BOTH
	// helm.sh/readiness-success and helm.sh/readiness-failure (drivers add
	// kube.WithCustomReadinessStatusReader for such batches).
	HasCustomReadiness bool
	// LeafGroups: names of groups in this batch with no dependents in this
	// chart level's DAG. Informational — display and future policy only.
	LeafGroups []string
}

// Manifests flattens the batch's groups in order. Convenience for drivers.
func (b Batch) Manifests() []releaseutil.Manifest {
	var manifests []releaseutil.Manifest
	for _, group := range b.Groups {
		manifests = append(manifests, group.Manifests...)
	}
	return manifests
}

// Warning is a non-fatal misconfiguration surfaced during Build.
// Drivers log them; lint promotes them to errors.
type Warning struct {
	ChartPath string // "" for chart-independent warnings
	Message   string
}

// ChartLevel summarizes one chart's slot in the walk, in pre-order.
// Consumed by `helm dag` (subchart-batch display) and diagnostics.
type ChartLevel struct {
	Path            string // manifest path-prefix (as Batch.ChartPath)
	Depth           int
	SubchartBatches [][]string // declared subcharts per parallel DAG batch (effective names)
	Undeclared      []string   // rendered subcharts absent from Chart.yaml deps (sorted)
	Unresolved      []string   // rendered subchart dirs with no resolvable chart object (sorted)
}

// Plan is the complete deployment sequence for one release's manifests.
type Plan struct {
	Batches  []Batch      // forward (install) order
	Levels   []ChartLevel // pre-order chart traversal
	Warnings []Warning
}

// Reverse returns a new Plan whose Batches are in exact reverse order, for
// uninstall and removed-resource deletion (HIP: uninstall is the exact reverse
// of install). Levels and Warnings are shared unchanged; Wait retains its
// meaning ("gate before next batch", i.e. wait-for-delete).
func (p *Plan) Reverse() *Plan {
	if p == nil {
		return nil
	}

	reversed := make([]Batch, len(p.Batches))
	for i, batch := range p.Batches {
		reversed[len(p.Batches)-1-i] = batch
	}

	return &Plan{
		Batches:  reversed,
		Levels:   p.Levels,
		Warnings: p.Warnings,
	}
}

// DisplayPath converts a manifest path-prefix to the HIP display form used by
// template markers and `helm dag`: "parent/charts/db/charts/redis" → "parent/db/redis".
// (Chart names cannot contain '/', so replacing "/charts/" is unambiguous;
// a subchart literally named "charts" is pathological and unsupported.)
func DisplayPath(chartPath string) string {
	return strings.ReplaceAll(chartPath, "/charts/", "/")
}
