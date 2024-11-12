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
	"strings"
	"testing"
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

const expectedManifest1 = `apiVersion: v1
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

const mockManifestFile2 = `

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

---apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo-2,
  annotations:
    "helm.sh/hook": test
spec:
  containers:
  - name: nemo-test
    image: fake-image
    cmd: fake-command
`

const expectedManifest2 = `apiVersion: v1
kind: Pod
metadata:
  name: finding-nemo-2,
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
	expected := map[string]string{"manifest-0": expectedManifest1}
	if !reflect.DeepEqual(manifests, expected) {
		t.Errorf("Expected \n%v\n got: \n%v", expected, manifests)
	}
}

func TestSplitManifestDocBreak(t *testing.T) {
	manifests := SplitManifests(mockManifestFile2)
	if len(manifests) != 2 {
		t.Errorf("Expected 2 manifest, got %v", len(manifests))
	}
	expected := map[string]string{"manifest-0": expectedManifest1, "manifest-1": expectedManifest2}
	if !reflect.DeepEqual(manifests, expected) {
		t.Errorf("Expected \n%v\n got: \n%v", expected, manifests)
	}

}

func TestSplitManifestNoDocBreak(t *testing.T) {
	manifests := SplitManifests(expectedManifest1)
	if len(manifests) != 1 {
		t.Errorf("Expected 1 manifest, got %v", len(manifests))
	}
	expected := map[string]string{"manifest-0": expectedManifest1}
	if !reflect.DeepEqual(manifests, expected) {
		t.Errorf("Expected \n%v\n got: \n%v", expected, manifests)
	}

}

func createManifest() string {
	sb := strings.Builder{}
	for i := 0; i < 10000; i++ {
		sb.WriteString(expectedManifest2)
		sb.WriteString("\n---")
	}
	return sb.String()
}

var BenchmarkSplitManifestsResult map[string]string
var largeManifest = createManifest()

func BenchmarkSplitManifests(b *testing.B) {
	var r map[string]string
	for n := 0; n < b.N; n++ {
		r = SplitManifests(largeManifest)
	}

	BenchmarkSplitManifestsResult = r
}
