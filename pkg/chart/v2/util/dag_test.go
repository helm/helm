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
	"strings"
	"testing"
)

func TestDAGGetBatches_Empty(t *testing.T) {
	d := NewDAG()
	batches, err := d.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 0 {
		t.Errorf("expected 0 batches, got %d", len(batches))
	}
}

func TestDAGGetBatches_SingleNode(t *testing.T) {
	d := NewDAG()
	d.AddNode("a")
	batches, err := d.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 1 || batches[0][0] != "a" {
		t.Errorf("expected batch [a], got %v", batches[0])
	}
}

func TestDAGGetBatches_Linear(t *testing.T) {
	// a → b → c (a must be deployed before b, b before c)
	d := NewDAG()
	d.AddNode("a")
	d.AddNode("b")
	d.AddNode("c")
	// AddEdge(from, to) means "to depends on from" (from must come before to)
	if err := d.AddEdge("a", "b"); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	if err := d.AddEdge("b", "c"); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}
	batches, err := d.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d: %v", len(batches), batches)
	}
	if !containsExactly(batches[0], "a") {
		t.Errorf("batch 0 should be [a], got %v", batches[0])
	}
	if !containsExactly(batches[1], "b") {
		t.Errorf("batch 1 should be [b], got %v", batches[1])
	}
	if !containsExactly(batches[2], "c") {
		t.Errorf("batch 2 should be [c], got %v", batches[2])
	}
}

func TestDAGGetBatches_Diamond(t *testing.T) {
	// a → b, a → c, b → d, c → d
	d := NewDAG()
	for _, n := range []string{"a", "b", "c", "d"} {
		d.AddNode(n)
	}
	if err := d.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("a", "c"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("b", "d"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("c", "d"); err != nil {
		t.Fatal(err)
	}
	batches, err := d.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d: %v", len(batches), batches)
	}
	if !containsExactly(batches[0], "a") {
		t.Errorf("batch 0 should be [a], got %v", batches[0])
	}
	if !containsAll(batches[1], "b", "c") {
		t.Errorf("batch 1 should be [b, c], got %v", batches[1])
	}
	if !containsExactly(batches[2], "d") {
		t.Errorf("batch 2 should be [d], got %v", batches[2])
	}
}

func TestDAGGetBatches_MultipleRoots(t *testing.T) {
	// a and b are roots, both → c
	d := NewDAG()
	for _, n := range []string{"a", "b", "c"} {
		d.AddNode(n)
	}
	if err := d.AddEdge("a", "c"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("b", "c"); err != nil {
		t.Fatal(err)
	}
	batches, err := d.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d: %v", len(batches), batches)
	}
	if !containsAll(batches[0], "a", "b") {
		t.Errorf("batch 0 should be [a, b], got %v", batches[0])
	}
	if !containsExactly(batches[1], "c") {
		t.Errorf("batch 1 should be [c], got %v", batches[1])
	}
}

func TestDAGGetBatches_Cycle(t *testing.T) {
	d := NewDAG()
	for _, n := range []string{"a", "b", "c"} {
		d.AddNode(n)
	}
	if err := d.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("b", "c"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEdge("c", "a"); err != nil {
		t.Fatal(err)
	}
	_, err := d.GetBatches()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestDAGAddEdge_UnknownNode(t *testing.T) {
	d := NewDAG()
	d.AddNode("a")
	// "b" is not registered
	err := d.AddEdge("a", "b")
	if err == nil {
		t.Fatal("expected error for unknown node, got nil")
	}
}

func TestDAGAddEdge_SelfLoop(t *testing.T) {
	d := NewDAG()
	d.AddNode("a")
	err := d.AddEdge("a", "a")
	if err == nil {
		t.Fatal("expected error for self-loop, got nil")
	}
}

// TestDAGGetBatches_NodesWithoutEdges tests that isolated nodes are put in batch 0.
func TestDAGGetBatches_NodesWithoutEdges(t *testing.T) {
	d := NewDAG()
	d.AddNode("a")
	d.AddNode("b")
	// a and b have no edges — both root nodes
	batches, err := d.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d: %v", len(batches), batches)
	}
	if !containsAll(batches[0], "a", "b") {
		t.Errorf("batch 0 should contain [a, b], got %v", batches[0])
	}
}

func TestDAGAddEdge_DuplicateIdempotent(t *testing.T) {
	d := NewDAG()
	d.AddNode("a")
	d.AddNode("b")
	if err := d.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	// Adding the same edge again should be a no-op.
	if err := d.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	batches, err := d.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be 2 batches: [a] then [b], NOT corrupted by double in-degree.
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d: %v", len(batches), batches)
	}
	if !containsExactly(batches[0], "a") {
		t.Errorf("batch 0 should be [a], got %v", batches[0])
	}
	if !containsExactly(batches[1], "b") {
		t.Errorf("batch 1 should be [b], got %v", batches[1])
	}
}

// helper: checks slice contains exactly these elements (order-independent)
func containsExactly(slice []string, items ...string) bool {
	if len(slice) != len(items) {
		return false
	}
	return containsAll(slice, items...)
}

// helper: checks slice contains all given items
func containsAll(slice []string, items ...string) bool {
	set := make(map[string]bool)
	for _, s := range slice {
		set[s] = true
	}
	for _, item := range items {
		if !set[item] {
			return false
		}
	}
	return len(slice) == len(items)
}
