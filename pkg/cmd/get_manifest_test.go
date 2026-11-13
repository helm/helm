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

package cmd

import (
	"strings"
	"testing"

	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestGetManifest(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "get manifest with release",
		cmd:    "get manifest juno",
		golden: "output/get-manifest.txt",
		rels:   []*release.Release{release.Mock(&release.MockReleaseOptions{Name: "juno"})},
	}, {
		name:      "get manifest without args",
		cmd:       "get manifest",
		golden:    "output/get-manifest-no-args.txt",
		wantError: true,
	}}
	runTestCmd(t, tests)
}

func TestGetManifestPrintsStoredManifestVerbatim(t *testing.T) {
	const annotationLine = `    helm.sh/depends-on/resource-groups: '["db"]'`
	manifest := `apiVersion: v1
kind: ConfigMap
metadata:
  name: sequenced
  annotations:
    helm.sh/resource-group: app
` + annotationLine + `
data:
  key: value
`
	rel := release.Mock(&release.MockReleaseOptions{Name: "sequenced"})
	rel.Manifest = manifest

	store := storageFixture()
	if err := store.Create(rel); err != nil {
		t.Fatal(err)
	}
	_, out, err := executeActionCommandC(store, "get manifest sequenced")
	if err != nil {
		t.Fatal(err)
	}

	// Invariant pin: get manifest prints the stored release record verbatim,
	// including sequencing annotations that template output strips.
	if !strings.Contains(out, "helm.sh/depends-on/resource-groups") {
		t.Fatalf("expected stored sequencing annotation key in output:\n%s", out)
	}
	if !strings.Contains(out, annotationLine) {
		t.Fatalf("expected exact stored sequencing annotation line in output:\n%s", out)
	}
}

func TestGetManifestCompletion(t *testing.T) {
	checkReleaseCompletion(t, "get manifest", false)
}

func TestGetManifestRevisionCompletion(t *testing.T) {
	revisionFlagCompletionTest(t, "get manifest")
}

func TestGetManifestFileCompletion(t *testing.T) {
	checkFileCompletion(t, "get manifest", false)
	checkFileCompletion(t, "get manifest myrelease", false)
}
