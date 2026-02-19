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
)

// makeManifest creates a minimal K8s resource YAML with the given annotations.
func makeManifest(name, sourcePath string, annotations map[string]string) Manifest {
	content := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n"
	if len(annotations) > 0 {
		content += "  annotations:\n"
		for k, v := range annotations {
			content += "    " + k + ": \"" + v + "\"\n"
		}
	}
	head := &SimpleHead{}
	head.Metadata = &struct {
		Name        string            `json:"name"`
		Annotations map[string]string `json:"annotations"`
	}{
		Name:        name,
		Annotations: annotations,
	}
	return Manifest{
		Name:    sourcePath,
		Content: content,
		Head:    head,
	}
}

func TestParseResourceGroups_NoAnnotations(t *testing.T) {
	manifests := []Manifest{
		makeManifest("cm1", "chart/templates/cm1.yaml", nil),
		makeManifest("cm2", "chart/templates/cm2.yaml", nil),
	}
	result, warnings := ParseResourceGroups(manifests)
	if len(result.Groups) != 0 {
		t.Errorf("expected no groups, got %v", result.Groups)
	}
	if len(result.Unsequenced) != 2 {
		t.Errorf("expected 2 unsequenced manifests, got %d", len(result.Unsequenced))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestParseResourceGroups_SingleGroup(t *testing.T) {
	manifests := []Manifest{
		makeManifest("svc", "chart/templates/svc.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
	}
	result, warnings := ParseResourceGroups(manifests)
	if len(result.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d: %v", len(result.Groups), result.Groups)
	}
	if _, ok := result.Groups["database"]; !ok {
		t.Errorf("expected group 'database', got %v", result.Groups)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestParseResourceGroups_GroupWithDependency(t *testing.T) {
	manifests := []Manifest{
		makeManifest("db", "chart/templates/db.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["database"]`,
		}),
	}
	result, warnings := ParseResourceGroups(manifests)
	if len(result.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result.Groups))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	dag, err := BuildResourceGroupDAG(result)
	if err != nil {
		t.Fatalf("BuildResourceGroupDAG: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("GetBatches: %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d: %v", len(batches), batches)
	}
	if batches[0][0] != "database" {
		t.Errorf("expected 'database' in batch 0, got %v", batches[0])
	}
	if batches[1][0] != "app" {
		t.Errorf("expected 'app' in batch 1, got %v", batches[1])
	}
}

func TestParseResourceGroups_RootGroupInBatch0(t *testing.T) {
	// A group with no depends-on is a root → batch 0, NOT unsequenced.
	manifests := []Manifest{
		makeManifest("db", "chart/templates/db.yaml", map[string]string{
			AnnotationResourceGroup: "standalone",
		}),
	}
	result, warnings := ParseResourceGroups(manifests)
	if len(result.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result.Groups))
	}
	if len(result.Unsequenced) != 0 {
		t.Errorf("root group should not be unsequenced, got %v", result.Unsequenced)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	dag, err := BuildResourceGroupDAG(result)
	if err != nil {
		t.Fatalf("BuildResourceGroupDAG: %v", err)
	}
	batches, err := dag.GetBatches()
	if err != nil {
		t.Fatalf("GetBatches: %v", err)
	}
	if len(batches) != 1 || batches[0][0] != "standalone" {
		t.Errorf("expected batch 0 = [standalone], got %v", batches)
	}
}

func TestParseResourceGroups_NonExistentGroupReference_Warning(t *testing.T) {
	// 'app' depends on 'nonexistent' which has no resources.
	manifests := []Manifest{
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `["nonexistent"]`,
		}),
	}
	result, warnings := ParseResourceGroups(manifests)
	if len(warnings) == 0 {
		t.Error("expected warning for non-existent group reference, got none")
	}
	// 'app' should be moved to unsequenced batch
	if len(result.Unsequenced) == 0 {
		t.Error("expected app to be in unsequenced batch")
	}
}

func TestParseResourceGroups_InvalidDependsOnJSON(t *testing.T) {
	manifests := []Manifest{
		makeManifest("app", "chart/templates/app.yaml", map[string]string{
			AnnotationResourceGroup:           "app",
			AnnotationDependsOnResourceGroups: `not-valid-json`,
		}),
	}
	result, warnings := ParseResourceGroups(manifests)
	if len(warnings) == 0 {
		t.Error("expected warning for invalid JSON, got none")
	}
	// app should go to unsequenced batch
	if len(result.Unsequenced) == 0 {
		t.Error("expected app in unsequenced batch")
	}
}

func TestParseResourceGroups_MixedSequencedAndUnsequenced(t *testing.T) {
	manifests := []Manifest{
		makeManifest("db", "chart/templates/db.yaml", map[string]string{
			AnnotationResourceGroup: "database",
		}),
		makeManifest("plain", "chart/templates/plain.yaml", nil), // no annotations
	}
	result, warnings := ParseResourceGroups(manifests)
	if len(result.Groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(result.Groups))
	}
	if len(result.Unsequenced) != 1 {
		t.Errorf("expected 1 unsequenced manifest, got %d", len(result.Unsequenced))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}
