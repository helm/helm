/*
Copyright 2017 The Kubernetes Authors All rights reserved.

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

package releaseutil

import (
	"testing"
)

func TestSplitManifestsWithHeads(t *testing.T) {
	rawManifest := `kind: TestKind
apiVersion: v1
metadata:
  name: TestName
  annotations:
    key1: value1
    key2: value2`

	rawManifests := rawManifest + "\n---\n" + rawManifest

	manifests, err := SplitManifestsWithHeads(rawManifests)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if len(manifests) != 2 {
		t.Errorf("Expected manifests length to be 2, got %d", len(manifests))
	}

	for _, m := range manifests {
		if m.SimpleHead.Kind != "TestKind" {
			t.Errorf("Expected Kind to be TestKind, got %s", m.SimpleHead.Kind)
		}
		if m.SimpleHead.Version != "v1" {
			t.Errorf("Expected Version to be v1, got %s", m.SimpleHead.Version)
		}
		if m.SimpleHead.Metadata.Name != "TestName" {
			t.Errorf("Expected Name to be TestName, got %s", m.SimpleHead.Metadata.Name)
		}
		if val1, ok := m.SimpleHead.Metadata.Annotations["key1"]; !ok || val1 != "value1" {
			t.Errorf("Expected annotation key1 to be value1, got %s", val1)
		}
		if val2, ok := m.SimpleHead.Metadata.Annotations["key2"]; !ok || val2 != "value2" {
			t.Errorf("Expected annotation key2 to be value2, got %s", val2)
		}
		if m.Content != rawManifest {
			t.Errorf("Expected Content to be equal to original, got %s", m.Content)
		}
	}
}
