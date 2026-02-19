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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGEmpty(t *testing.T) {
	dag := NewDAG()
	assert.Equal(t, 0, dag.Len())

	sorted, err := dag.TopologicalSort()
	require.NoError(t, err)
	assert.Empty(t, sorted)

	batches, err := dag.Batches()
	require.NoError(t, err)
	assert.Nil(t, batches)
}

func TestDAGSingleNode(t *testing.T) {
	dag := NewDAG()
	dag.AddNode("nginx")

	assert.Equal(t, 1, dag.Len())
	assert.True(t, dag.HasNode("nginx"))

	sorted, err := dag.TopologicalSort()
	require.NoError(t, err)
	assert.Equal(t, []string{"nginx"}, sorted)

	batches, err := dag.Batches()
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"nginx"}}, batches)
}

func TestDAGLinearChain(t *testing.T) {
	// A -> B -> C (C depends on B, B depends on A)
	dag := NewDAG()
	require.NoError(t, dag.AddEdge("A", "B"))
	require.NoError(t, dag.AddEdge("B", "C"))

	sorted, err := dag.TopologicalSort()
	require.NoError(t, err)
	assert.Equal(t, []string{"A", "B", "C"}, sorted)

	batches, err := dag.Batches()
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"A"}, {"B"}, {"C"}}, batches)
}

func TestDAGDiamondDependency(t *testing.T) {
	// HIP-0025 example: nginx + rabbitmq -> bar -> foo
	dag := NewDAG()
	require.NoError(t, dag.AddEdge("nginx", "bar"))
	require.NoError(t, dag.AddEdge("rabbitmq", "bar"))
	require.NoError(t, dag.AddEdge("bar", "foo"))
	require.NoError(t, dag.AddEdge("rabbitmq", "foo"))

	sorted, err := dag.TopologicalSort()
	require.NoError(t, err)
	// nginx and rabbitmq have no deps, come first (alphabetical)
	assert.Equal(t, "nginx", sorted[0])
	assert.Equal(t, "rabbitmq", sorted[1])
	// bar depends on both, comes next
	assert.Equal(t, "bar", sorted[2])
	// foo depends on bar and rabbitmq
	assert.Equal(t, "foo", sorted[3])

	batches, err := dag.Batches()
	require.NoError(t, err)
	require.Len(t, batches, 3)
	assert.Equal(t, []string{"nginx", "rabbitmq"}, batches[0])
	assert.Equal(t, []string{"bar"}, batches[1])
	assert.Equal(t, []string{"foo"}, batches[2])
}

func TestDAGParallelNoDeps(t *testing.T) {
	dag := NewDAG()
	dag.AddNode("a")
	dag.AddNode("b")
	dag.AddNode("c")

	batches, err := dag.Batches()
	require.NoError(t, err)
	require.Len(t, batches, 1)
	assert.Equal(t, []string{"a", "b", "c"}, batches[0])
}

func TestDAGCircularDependency(t *testing.T) {
	dag := NewDAG()
	require.NoError(t, dag.AddEdge("A", "B"))
	require.NoError(t, dag.AddEdge("B", "C"))
	require.NoError(t, dag.AddEdge("C", "A"))

	err := dag.DetectCycles()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")

	_, err = dag.TopologicalSort()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")
}

func TestDAGSelfDependency(t *testing.T) {
	dag := NewDAG()
	err := dag.AddEdge("A", "A")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self-dependency detected")
}

func TestDAGDependsOnAndDependents(t *testing.T) {
	dag := NewDAG()
	require.NoError(t, dag.AddEdge("db", "app"))
	require.NoError(t, dag.AddEdge("cache", "app"))

	deps := dag.DependsOn("app")
	assert.Equal(t, []string{"cache", "db"}, deps)

	dependents := dag.Dependents("db")
	assert.Equal(t, []string{"app"}, dependents)
}

func TestDAGOrphanedNodes(t *testing.T) {
	// Nodes with no edges should appear in first batch
	dag := NewDAG()
	dag.AddNode("orphan")
	require.NoError(t, dag.AddEdge("A", "B"))

	batches, err := dag.Batches()
	require.NoError(t, err)
	require.Len(t, batches, 2)
	// First batch: A and orphan (no deps)
	assert.Equal(t, []string{"A", "orphan"}, batches[0])
	assert.Equal(t, []string{"B"}, batches[1])
}

func TestDAGComplexGraph(t *testing.T) {
	// Complex HIP-0025 scenario:
	// database and queue have no deps
	// app depends on database and queue
	// queue depends on another-group (missing, but we add it)
	dag := NewDAG()
	dag.AddNode("database")
	dag.AddNode("queue")
	dag.AddNode("another-group")
	require.NoError(t, dag.AddEdge("another-group", "queue"))
	require.NoError(t, dag.AddEdge("database", "app"))
	require.NoError(t, dag.AddEdge("queue", "app"))

	batches, err := dag.Batches()
	require.NoError(t, err)
	require.Len(t, batches, 3)
	assert.Equal(t, []string{"another-group", "database"}, batches[0])
	assert.Equal(t, []string{"queue"}, batches[1])
	assert.Equal(t, []string{"app"}, batches[2])
}
