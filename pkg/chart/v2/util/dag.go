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

package util

import (
	"fmt"
	"sort"
	"strings"
)

// DAG is a directed acyclic graph of string-keyed nodes, used for resource-group
// and subchart dependency ordering.
//
// Edges are directed: AddEdge("a", "b") means "b depends on a" (a must come before b).
type DAG struct {
	nodes    map[string]struct{}
	edges    map[string][]string // from -> []to (dependents)
	edgeSet  map[string]struct{} // "from\x00to" -> exists (dedup guard)
	inDegree map[string]int      // node -> number of prerequisites
}

// NewDAG creates an empty DAG.
func NewDAG() *DAG {
	return &DAG{
		nodes:    make(map[string]struct{}),
		edges:    make(map[string][]string),
		edgeSet:  make(map[string]struct{}),
		inDegree: make(map[string]int),
	}
}

// AddNode registers a node in the DAG. Duplicate adds are idempotent.
func (d *DAG) AddNode(name string) {
	if _, ok := d.nodes[name]; ok {
		return
	}
	d.nodes[name] = struct{}{}
	d.edges[name] = nil
	d.inDegree[name] = 0
}

// AddEdge adds a directed edge: "to" depends on "from" (from is deployed before to).
// Returns an error if either node is unknown or if a self-loop is requested.
func (d *DAG) AddEdge(from, to string) error {
	if from == to {
		return fmt.Errorf("self-loop not allowed: %q", from)
	}
	if _, ok := d.nodes[from]; !ok {
		return fmt.Errorf("unknown node %q", from)
	}
	if _, ok := d.nodes[to]; !ok {
		return fmt.Errorf("unknown node %q", to)
	}
	// Silently skip duplicate edges to prevent in-degree corruption.
	key := from + "\x00" + to
	if _, exists := d.edgeSet[key]; exists {
		return nil
	}
	d.edgeSet[key] = struct{}{}
	d.edges[from] = append(d.edges[from], to)
	d.inDegree[to]++
	return nil
}

// GetBatches performs a topological sort using Kahn's algorithm and returns
// the nodes grouped into deployment batches. Each batch contains nodes that
// can be deployed in parallel. Batches are ordered: batch 0 has no prerequisites,
// batch 1 depends only on batch 0, etc.
//
// Returns an error if a cycle is detected, including the names of the nodes
// involved in the cycle.
func (d *DAG) GetBatches() ([][]string, error) {
	if len(d.nodes) == 0 {
		return nil, nil
	}

	// Copy in-degrees so we don't mutate the DAG.
	inDeg := make(map[string]int, len(d.inDegree))
	for k, v := range d.inDegree {
		inDeg[k] = v
	}

	var batches [][]string
	processed := 0

	for {
		// Collect all nodes with zero in-degree.
		var batch []string
		for n := range d.nodes {
			if inDeg[n] == 0 {
				batch = append(batch, n)
				inDeg[n] = -1 // mark as queued
			}
		}
		if len(batch) == 0 {
			break
		}
		// Sort for deterministic output.
		sort.Strings(batch)
		batches = append(batches, batch)
		processed += len(batch)

		// Reduce in-degree for all dependents.
		for _, n := range batch {
			for _, dep := range d.edges[n] {
				inDeg[dep]--
			}
		}
	}

	if processed < len(d.nodes) {
		// Nodes still with positive in-degree are part of a cycle.
		var cycleNodes []string
		for n := range d.nodes {
			if inDeg[n] > 0 {
				cycleNodes = append(cycleNodes, n)
			}
		}
		sort.Strings(cycleNodes)
		return nil, fmt.Errorf("cycle detected among nodes: %s", strings.Join(cycleNodes, ", "))
	}

	return batches, nil
}

// Nodes returns a sorted slice of all node names in the DAG.
func (d *DAG) Nodes() []string {
	ns := make([]string, 0, len(d.nodes))
	for n := range d.nodes {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// HasNode reports whether the DAG contains a node with the given name.
func (d *DAG) HasNode(name string) bool {
	_, ok := d.nodes[name]
	return ok
}
