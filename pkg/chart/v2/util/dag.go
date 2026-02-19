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

// DAG is a directed acyclic graph for dependency resolution.
// Nodes represent subcharts or resource-groups. Edges represent
// "must be ready before" relationships.
type DAG struct {
	// nodes is the set of all node names in the graph.
	nodes map[string]bool
	// edges maps a node to the set of nodes it depends on (upstream).
	// If edges["app"] = {"db": true}, then "db" must be ready before "app".
	edges map[string]map[string]bool
}

// NewDAG creates an empty DAG.
func NewDAG() *DAG {
	return &DAG{
		nodes: make(map[string]bool),
		edges: make(map[string]map[string]bool),
	}
}

// AddNode adds a node to the graph. Idempotent.
func (d *DAG) AddNode(name string) {
	d.nodes[name] = true
}

// AddEdge adds a directed edge: "from" must be ready before "to" can start.
// Both nodes are implicitly added if not present.
// Returns an error if from == to (self-cycle).
func (d *DAG) AddEdge(from, to string) error {
	if from == to {
		return fmt.Errorf("self-dependency detected: %q depends on itself", from)
	}
	d.AddNode(from)
	d.AddNode(to)
	if d.edges[to] == nil {
		d.edges[to] = make(map[string]bool)
	}
	d.edges[to][from] = true
	return nil
}

// DetectCycles checks for circular dependencies using DFS.
// Returns a descriptive error if a cycle is found, nil otherwise.
func (d *DAG) DetectCycles() error {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	state := make(map[string]int, len(d.nodes))
	path := make([]string, 0)

	var visit func(node string) error
	visit = func(node string) error {
		state[node] = visiting
		path = append(path, node)

		for dep := range d.edges[node] {
			switch state[dep] {
			case visiting:
				// Found cycle — build cycle description
				cycleStart := -1
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				cycle := append(path[cycleStart:], dep)
				return fmt.Errorf("circular dependency detected: %s", strings.Join(cycle, " -> "))
			case unvisited:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		path = path[:len(path)-1]
		state[node] = visited
		return nil
	}

	// Visit in sorted order for deterministic error messages
	sorted := d.sortedNodes()
	for _, node := range sorted {
		if state[node] == unvisited {
			if err := visit(node); err != nil {
				return err
			}
		}
	}
	return nil
}

// TopologicalSort returns nodes in dependency order: nodes with no
// dependencies come first. Nodes at the same level are sorted
// alphabetically for determinism.
func (d *DAG) TopologicalSort() ([]string, error) {
	if err := d.DetectCycles(); err != nil {
		return nil, err
	}

	inDegree := make(map[string]int, len(d.nodes))
	for node := range d.nodes {
		inDegree[node] = len(d.edges[node])
	}

	// Collect nodes with no incoming edges
	var queue []string
	for node, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// For each node that depends on this one, decrement in-degree
		for downstream := range d.nodes {
			if d.edges[downstream][node] {
				inDegree[downstream]--
				if inDegree[downstream] == 0 {
					queue = append(queue, downstream)
					sort.Strings(queue)
				}
			}
		}
	}

	return result, nil
}

// Batches returns nodes grouped by dependency level (topological layers).
// Each batch can be deployed in parallel; batches must be deployed sequentially.
// Batch 0 has no dependencies, batch 1 depends only on batch 0, etc.
func (d *DAG) Batches() ([][]string, error) {
	if err := d.DetectCycles(); err != nil {
		return nil, err
	}

	if len(d.nodes) == 0 {
		return nil, nil
	}

	inDegree := make(map[string]int, len(d.nodes))
	for node := range d.nodes {
		inDegree[node] = len(d.edges[node])
	}

	remaining := make(map[string]bool)
	for node := range d.nodes {
		remaining[node] = true
	}

	var batches [][]string
	for len(remaining) > 0 {
		// Find all nodes with in-degree 0 among remaining
		var batch []string
		for node := range remaining {
			if inDegree[node] == 0 {
				batch = append(batch, node)
			}
		}

		if len(batch) == 0 {
			// Should not happen after cycle check, but guard against it
			return nil, fmt.Errorf("internal error: no nodes with in-degree 0 among remaining nodes")
		}

		sort.Strings(batch)
		batches = append(batches, batch)

		// Remove batch nodes and update in-degrees
		for _, node := range batch {
			delete(remaining, node)
			for downstream := range remaining {
				if d.edges[downstream][node] {
					inDegree[downstream]--
				}
			}
		}
	}

	return batches, nil
}

// Nodes returns all node names in the DAG.
func (d *DAG) Nodes() []string {
	return d.sortedNodes()
}

// DependsOn returns the set of nodes that the given node depends on.
func (d *DAG) DependsOn(node string) []string {
	deps := d.edges[node]
	result := make([]string, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	sort.Strings(result)
	return result
}

// Dependents returns the set of nodes that depend on the given node.
func (d *DAG) Dependents(node string) []string {
	var result []string
	for n, deps := range d.edges {
		if deps[node] {
			result = append(result, n)
		}
	}
	sort.Strings(result)
	return result
}

// HasNode returns true if the node exists in the graph.
func (d *DAG) HasNode(name string) bool {
	return d.nodes[name]
}

// Len returns the number of nodes.
func (d *DAG) Len() int {
	return len(d.nodes)
}

func (d *DAG) sortedNodes() []string {
	result := make([]string, 0, len(d.nodes))
	for n := range d.nodes {
		result = append(result, n)
	}
	sort.Strings(result)
	return result
}
