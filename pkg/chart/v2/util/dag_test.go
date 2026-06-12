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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAG_BatchOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nodes    []string
		edges    [][2]string
		expected [][]string
	}{
		{
			name:     "empty DAG",
			expected: nil,
		},
		{
			name:     "single node",
			nodes:    []string{"a"},
			expected: [][]string{{"a"}},
		},
		{
			name:  "linear chain",
			nodes: []string{"a", "b", "c"},
			edges: [][2]string{{"a", "b"}, {"b", "c"}},
			expected: [][]string{
				{"a"},
				{"b"},
				{"c"},
			},
		},
		{
			name:  "diamond",
			nodes: []string{"a", "b", "c", "d"},
			edges: [][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
			expected: [][]string{
				{"a"},
				{"b", "c"},
				{"d"},
			},
		},
		{
			name:  "multiple roots",
			nodes: []string{"a", "b", "c"},
			edges: [][2]string{{"a", "c"}, {"b", "c"}},
			expected: [][]string{
				{"a", "b"},
				{"c"},
			},
		},
		{
			name:  "duplicate edge is idempotent",
			nodes: []string{"a", "b"},
			edges: [][2]string{{"a", "b"}, {"a", "b"}},
			expected: [][]string{
				{"a"},
				{"b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dag := buildDAG(t, tt.nodes, tt.edges)

			batches, err := dag.GetBatches()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, batches)
		})
	}
}

func TestDAG_CycleDetection(t *testing.T) {
	t.Parallel()

	dag := buildDAG(t, []string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}})

	batches, err := dag.GetBatches()
	require.Error(t, err)
	assert.Nil(t, batches)
	assert.ErrorContains(t, err, "cycle")
	assert.ErrorContains(t, err, "a")
	assert.ErrorContains(t, err, "b")
	assert.ErrorContains(t, err, "c")
}

func TestDAG_SelfLoop(t *testing.T) {
	t.Parallel()

	dag := NewDAG()
	dag.AddNode("a")

	err := dag.AddEdge("a", "a")
	require.Error(t, err)
	assert.ErrorContains(t, err, "self-loop")
}

func TestDAG_UnknownNode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		from  string
		to    string
		match string
	}{
		{
			name:  "missing source node",
			from:  "missing",
			to:    "a",
			match: "unknown node",
		},
		{
			name:  "missing destination node",
			from:  "a",
			to:    "missing",
			match: "unknown node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dag := NewDAG()
			dag.AddNode("a")

			err := dag.AddEdge(tt.from, tt.to)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.match)
		})
	}
}

func TestDAG_DisconnectedComponents(t *testing.T) {
	t.Parallel()

	dag := buildDAG(t,
		[]string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"},
		[][2]string{{"alpha", "gamma"}, {"beta", "delta"}, {"delta", "epsilon"}},
	)

	batches, err := dag.GetBatches()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"alpha", "beta", "zeta"},
		{"delta", "gamma"},
		{"epsilon"},
	}, batches)
}

func TestDAG_LargeGraph(t *testing.T) {
	t.Parallel()

	const nodeCount = 128

	nodes := make([]string, 0, nodeCount)
	edges := make([][2]string, 0, nodeCount-1)
	for i := range nodeCount {
		nodes = append(nodes, fmt.Sprintf("node-%03d", i))
		if i == 0 {
			continue
		}
		edges = append(edges, [2]string{
			fmt.Sprintf("node-%03d", i-1),
			fmt.Sprintf("node-%03d", i),
		})
	}

	dag := buildDAG(t, nodes, edges)

	batches, err := dag.GetBatches()
	require.NoError(t, err)
	require.Len(t, batches, nodeCount)

	for i, batch := range batches {
		assert.Equal(t, []string{fmt.Sprintf("node-%03d", i)}, batch)
	}
}

func TestDAG_NodeHelpers(t *testing.T) {
	t.Parallel()

	dag := NewDAG()
	dag.AddNode("charlie")
	dag.AddNode("alpha")
	dag.AddNode("bravo")

	assert.True(t, dag.HasNode("alpha"))
	assert.False(t, dag.HasNode("delta"))
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, dag.Nodes())
}

func buildDAG(t *testing.T, nodes []string, edges [][2]string) *DAG {
	t.Helper()

	dag := NewDAG()
	for _, node := range nodes {
		dag.AddNode(node)
	}

	for _, edge := range edges {
		require.NoError(t, dag.AddEdge(edge[0], edge[1]))
	}

	return dag
}
