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

// Package sequence computes ordered deployment plans for release manifests.
package sequence

import (
	"strings"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

// BatchKind distinguishes the batch flavors a chart level produces.
type BatchKind uint8

const (
	// BatchKindGroups is one topological level of the resource-group DAG.
	BatchKindGroups BatchKind = iota
	// BatchKindUnsequenced is a chart level's trailing unsequenced batch.
	BatchKindUnsequenced
)

// Group is a named set of manifests deployed as one unit within a batch.
type Group struct {
	Name      string
	Manifests []releaseutil.Manifest
}

// Batch is one apply unit in a deployment plan. Wait marks the end of a stage:
// the executor waits for every resource applied since the previous stage
// before it continues. Batches of independent subcharts share a stage, so only
// the last batch of such a stage has Wait set.
type Batch struct {
	ChartPath  string
	Depth      int
	Kind       BatchKind
	Groups     []Group
	Manifests  []releaseutil.Manifest
	Wait       bool
	LeafGroups []string
}

// ChartLevel describes one chart in the plan's traversal order.
type ChartLevel struct {
	Path            string
	Depth           int
	SubchartBatches [][]string
	ParentDependsOn []string
	Undeclared      []string
	Unresolved      []string
}

// WarningKind classifies a non-fatal plan warning.
type WarningKind uint8

const (
	// WarningKindUndeclaredSubchart identifies rendered subcharts absent from Chart.yaml.
	WarningKindUndeclaredSubchart WarningKind = iota
	// WarningKindUnresolvedSubchart identifies rendered subcharts with no resolvable chart object.
	WarningKindUnresolvedSubchart
	// WarningKindResourceGroupDemotion identifies a resource or group demoted to the unsequenced batch.
	WarningKindResourceGroupDemotion
	// WarningKindIsolatedGroup identifies an edge-free group demoted to the unsequenced batch.
	WarningKindIsolatedGroup
)

// Warning is a non-fatal issue discovered while building a plan.
type Warning struct {
	Kind      WarningKind
	ChartPath string
	Message   string
}

// Plan is the complete deployment sequence for a release's manifests.
type Plan struct {
	Batches  []Batch
	Levels   []ChartLevel
	Warnings []Warning
}

// Reverse returns a new Plan with batches in exact reverse order. Stages keep
// their boundaries, so Wait moves to the last batch of each reversed stage.
// Levels and Warnings are shared unchanged.
func (p *Plan) Reverse() *Plan {
	if p == nil {
		return nil
	}

	reversed := make([]Batch, len(p.Batches))
	for i, batch := range p.Batches {
		batch.Wait = i == 0 || p.Batches[i-1].Wait
		reversed[len(p.Batches)-1-i] = batch
	}

	return &Plan{
		Batches:  reversed,
		Levels:   p.Levels,
		Warnings: p.Warnings,
	}
}

// DisplayPath converts a manifest path prefix to its compact chart path.
func DisplayPath(chartPath string) string {
	return strings.ReplaceAll(chartPath, "/charts/", "/")
}
