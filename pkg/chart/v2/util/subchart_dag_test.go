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

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func makeChart(name string, deps ...*chart.Dependency) *chart.Chart {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name: name,
		},
	}
	c.Metadata.Dependencies = deps
	return c
}

// dep creates an enabled dependency. In Helm's runtime, processDependencyEnabled
// sets Enabled=true for all deps before condition evaluation; we mirror that here.
func dep(name string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Enabled:   true,
		DependsOn: dependsOn,
	}
}

func depAlias(name, alias string, dependsOn ...string) *chart.Dependency {
	return &chart.Dependency{
		Name:      name,
		Alias:     alias,
		Enabled:   true,
		DependsOn: dependsOn,
	}
}

func TestBuildSubchartDAG_Empty(t *testing.T) {
	c := makeChart("parent")
	dag, err := BuildSubchartDAG(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 0 {
		t.Errorf("expected 0 batches, got %d", len(batches))
	}
}

func TestBuildSubchartDAG_NoDependencies(t *testing.T) {
	// Three subcharts with no ordering declarations — all in batch 0.
	c := makeChart("parent",
		dep("nginx"),
		dep("rabbitmq"),
		dep("postgres"),
	)
	dag, err := BuildSubchartDAG(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch, got %d: %v", len(batches), batches)
	}
	if !containsAll(batches[0], "nginx", "rabbitmq", "postgres") {
		t.Errorf("expected all subcharts in batch 0, got %v", batches[0])
	}
}

func TestBuildSubchartDAG_LinearOrder(t *testing.T) {
	// postgres → rabbitmq → app
	c := makeChart("parent",
		dep("postgres"),
		dep("rabbitmq", "postgres"),
		dep("app", "rabbitmq"),
	)
	dag, err := BuildSubchartDAG(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d: %v", len(batches), batches)
	}
}

func TestBuildSubchartDAG_AliasResolution(t *testing.T) {
	// "db" is aliased as "primary-db". "app" depends on "primary-db" (the alias).
	c := makeChart("parent",
		depAlias("postgres", "primary-db"),
		dep("app", "primary-db"),
	)
	dag, err := BuildSubchartDAG(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d: %v", len(batches), batches)
	}
	if !containsExactly(batches[0], "primary-db") {
		t.Errorf("expected [primary-db] in batch 0, got %v", batches[0])
	}
	if !containsExactly(batches[1], "app") {
		t.Errorf("expected [app] in batch 1, got %v", batches[1])
	}
}

func TestBuildSubchartDAG_NonExistentReference(t *testing.T) {
	// "app" depends on "nonexistent" which is not in the deps list.
	c := makeChart("parent",
		dep("app", "nonexistent"),
	)
	_, err := BuildSubchartDAG(c)
	if err == nil {
		t.Fatal("expected error for non-existent subchart reference, got nil")
	}
}

func TestBuildSubchartDAG_DisabledSubchart(t *testing.T) {
	// "app" depends on "cache", but "cache" is disabled.
	// The dependency edge should be silently removed (app still deploys).
	c := makeChart("parent",
		&chart.Dependency{
			Name:    "cache",
			Enabled: false, // disabled
		},
		dep("app", "cache"),
	)
	dag, err := BuildSubchartDAG(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// app should be in batch 0 since cache is disabled (edge removed)
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch (cache disabled), got %d: %v", len(batches), batches)
	}
	if !containsExactly(batches[0], "app") {
		t.Errorf("expected [app] in batch 0, got %v", batches[0])
	}
}

func TestBuildSubchartDAG_CycleDetection(t *testing.T) {
	c := makeChart("parent",
		dep("a", "b"),
		dep("b", "c"),
		dep("c", "a"),
	)
	dag, err := BuildSubchartDAG(c)
	if err != nil {
		t.Fatalf("unexpected error building DAG: %v", err)
	}
	_, err = dag.GetBatches()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestBuildSubchartDAG_AnnotationBased(t *testing.T) {
	// Uses helm.sh/depends-on/subcharts annotation format: "nginx depends-on postgres"
	// The annotation format is: subchart-name: depends-on-list (comma-separated)
	c := makeChart("parent",
		dep("postgres"),
		dep("nginx"),
	)
	// Set annotation: nginx depends on postgres
	c.Metadata.Annotations = map[string]string{
		"helm.sh/depends-on/subcharts": `{"nginx": ["postgres"]}`,
	}
	dag, err := BuildSubchartDAG(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d: %v", len(batches), batches)
	}
	if !containsExactly(batches[0], "postgres") {
		t.Errorf("expected [postgres] in batch 0, got %v", batches[0])
	}
	if !containsExactly(batches[1], "nginx") {
		t.Errorf("expected [nginx] in batch 1, got %v", batches[1])
	}
}
