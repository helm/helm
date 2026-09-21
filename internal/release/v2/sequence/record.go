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

package sequence

import (
	"errors"
	"fmt"

	releaseutil "helm.sh/helm/v4/internal/release/v2/manifest"
)

const planRecordVersion = 1

// PlanRecord is the JSON-safe representation of a deployment plan.
type PlanRecord struct {
	Version int           `json:"version"`
	Levels  []LevelRecord `json:"levels"`
	Batches []BatchRecord `json:"batches"`
}

// LevelRecord preserves one chart level's resolved sequencing metadata.
type LevelRecord struct {
	Path            string     `json:"path"`
	Depth           int        `json:"depth"`
	SubchartBatches [][]string `json:"subchartBatches,omitempty"`
	Edges           []Edge     `json:"edges,omitempty"`
	ParentDependsOn []string   `json:"parentDependsOn,omitempty"`
	Undeclared      []string   `json:"undeclared,omitempty"`
	Unresolved      []string   `json:"unresolved,omitempty"`
}

// Edge records one resolved dependency between effective subchart names.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// BatchRecord preserves one apply batch and its manifest membership.
type BatchRecord struct {
	ChartPath string        `json:"chartPath"`
	Depth     int           `json:"depth"`
	Kind      string        `json:"kind"`
	Barrier   bool          `json:"barrier"`
	Manifests []ManifestRef `json:"manifests"`
}

// ManifestRef identifies a stored manifest by position and path.
type ManifestRef struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
}

// NewPlanRecord converts a deployment plan into its lossless persisted form.
func NewPlanRecord(plan *Plan, manifests []releaseutil.Manifest) (*PlanRecord, error) {
	if plan == nil {
		return nil, errors.New("cannot persist a nil sequencing plan")
	}

	record := &PlanRecord{
		Version: planRecordVersion,
		Levels:  make([]LevelRecord, len(plan.Levels)),
		Batches: make([]BatchRecord, len(plan.Batches)),
	}
	for i, level := range plan.Levels {
		record.Levels[i] = LevelRecord{
			Path:            level.Path,
			Depth:           level.Depth,
			SubchartBatches: level.SubchartBatches,
			Edges:           levelBatchEdges(level.SubchartBatches),
			ParentDependsOn: level.ParentDependsOn,
			Undeclared:      level.Undeclared,
			Unresolved:      level.Unresolved,
		}
	}

	indexesByPath := make(map[string][]int, len(manifests))
	for i, manifest := range manifests {
		indexesByPath[manifest.Name] = append(indexesByPath[manifest.Name], i)
	}
	consumedByPath := make(map[string]int, len(indexesByPath))
	consumed := make([]bool, len(manifests))
	for i, batch := range plan.Batches {
		batchRecord := BatchRecord{
			ChartPath: batch.ChartPath,
			Depth:     batch.Depth,
			Kind:      "unsequenced",
			Barrier:   batch.Wait,
			Manifests: make([]ManifestRef, len(batch.Manifests)),
		}
		for j, manifest := range batch.Manifests {
			pathIndexes := indexesByPath[manifest.Name]
			pathOffset := consumedByPath[manifest.Name]
			if pathOffset >= len(pathIndexes) {
				return nil, fmt.Errorf("manifest %q in batch %d is not present in the stored manifest stream", manifest.Name, i)
			}
			index := pathIndexes[pathOffset]
			consumedByPath[manifest.Name]++
			consumed[index] = true
			batchRecord.Manifests[j] = ManifestRef{Index: index, Path: manifest.Name}
		}
		record.Batches[i] = batchRecord
	}
	for index, used := range consumed {
		if !used {
			return nil, fmt.Errorf("manifest %q at index %d is absent from the sequencing plan", manifests[index].Name, index)
		}
	}

	return record, nil
}

// Restore reconstructs a deployment plan after validating every manifest
// reference against the parsed stored stream.
func (record *PlanRecord) Restore(manifests []releaseutil.Manifest) (*Plan, error) {
	if record == nil {
		return nil, errors.New("cannot restore a nil sequencing plan record")
	}
	if record.Version != planRecordVersion {
		return nil, fmt.Errorf("unsupported plan record version %d", record.Version)
	}

	plan := &Plan{
		Levels:  make([]ChartLevel, len(record.Levels)),
		Batches: make([]Batch, len(record.Batches)),
	}
	for i, level := range record.Levels {
		plan.Levels[i] = ChartLevel{
			Path:            level.Path,
			Depth:           level.Depth,
			SubchartBatches: level.SubchartBatches,
			ParentDependsOn: level.ParentDependsOn,
			Undeclared:      level.Undeclared,
			Unresolved:      level.Unresolved,
		}
	}

	used := make([]bool, len(manifests))
	for i, batch := range record.Batches {
		if batch.Kind != "unsequenced" {
			return nil, fmt.Errorf("unsupported batch kind %q in batch %d", batch.Kind, i)
		}
		restored := Batch{
			ChartPath: batch.ChartPath,
			Depth:     batch.Depth,
			Wait:      batch.Barrier,
			Manifests: make([]releaseutil.Manifest, len(batch.Manifests)),
		}
		for j, ref := range batch.Manifests {
			if ref.Index < 0 || ref.Index >= len(manifests) {
				return nil, fmt.Errorf("manifest index %d out of range in batch %d", ref.Index, i)
			}
			if manifests[ref.Index].Name != ref.Path {
				return nil, fmt.Errorf("manifest path mismatch at index %d: record has %q, stored manifest has %q", ref.Index, ref.Path, manifests[ref.Index].Name)
			}
			if used[ref.Index] {
				return nil, fmt.Errorf("manifest index %d is referenced more than once", ref.Index)
			}
			used[ref.Index] = true
			restored.Manifests[j] = manifests[ref.Index]
		}
		plan.Batches[i] = restored
	}
	for index, included := range used {
		if !included {
			return nil, fmt.Errorf("manifest %q at index %d is absent from the plan record", manifests[index].Name, index)
		}
	}

	return plan, nil
}

func levelBatchEdges(batches [][]string) []Edge {
	var edges []Edge
	for i := 0; i+1 < len(batches); i++ {
		for _, from := range batches[i] {
			for _, to := range batches[i+1] {
				edges = append(edges, Edge{From: from, To: to})
			}
		}
	}
	return edges
}
