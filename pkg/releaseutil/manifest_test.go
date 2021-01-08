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

package releaseutil // import "helm.sh/helm/v3/pkg/releaseutil"

import (
	"reflect"
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

const mockManifestFile = `

---
apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo,
  annotations:
    "helm.sh/hook": test
spec:
  containers:
  - name: nemo-test
    image: fake-image
    cmd: fake-command
`

const expectedManifest = `apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo,
  annotations:
    "helm.sh/hook": test
spec:
  containers:
  - name: nemo-test
    image: fake-image
    cmd: fake-command`

func TestSplitManifest(t *testing.T) {
	manifests := SplitManifests(mockManifestFile)
	if len(manifests) != 1 {
		t.Errorf("Expected 1 manifest, got %v", len(manifests))
	}
	expected := map[string]string{"manifest-0": expectedManifest}
	if !reflect.DeepEqual(manifests, expected) {
		t.Errorf("Expected %v, got %v", expected, manifests)
	}
}

func TestSplitAllManifests(t *testing.T) {
	mockRelease := release.Mock(&release.MockReleaseOptions{})
	manifests := SplitAllManifests(mockRelease)
	if len(manifests) != 2 {
		t.Errorf("Expected 2 manifests (including one containing a hook), got %v", len(manifests))
	}
	if len(mockRelease.Hooks) != 1 {
		t.Errorf("Expected 1 hook, got %v", len(mockRelease.Hooks))
	}
	manifestsLessHooks := SplitManifests(mockRelease.Manifest)
	if len(manifestsLessHooks) != 1 {
		t.Errorf("Expected 1 manifest without a hook, got %v", len(manifestsLessHooks))
	}
}
