/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package tiller

import (
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/release"
	util "k8s.io/helm/pkg/releaseutil"
)

func TestSortManifests(t *testing.T) {

	data := []struct {
		name     string
		path     string
		kind     string
		hooks    []release.Hook_Event
		manifest string
	}{
		{
			name:  "first",
			path:  "one",
			kind:  "Job",
			hooks: []release.Hook_Event{release.Hook_PRE_INSTALL},
			manifest: `apiVersion: v1
kind: Job
metadata:
  name: first
  labels:
    doesnot: matter
  annotations:
    "helm.sh/hook": pre-install
`,
		},
		{
			name:  "second",
			path:  "two",
			kind:  "ReplicaSet",
			hooks: []release.Hook_Event{release.Hook_POST_INSTALL},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: second
  annotations:
    "helm.sh/hook": post-install
`,
		}, {
			name:  "third",
			path:  "three",
			kind:  "ReplicaSet",
			hooks: []release.Hook_Event{},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: third
  annotations:
    "helm.sh/hook": no-such-hook
`,
		}, {
			name:  "fourth",
			path:  "four",
			kind:  "Pod",
			hooks: []release.Hook_Event{},
			manifest: `kind: Pod
apiVersion: v1
metadata:
  name: fourth
  annotations:
    nothing: here
`,
		}, {
			name:  "fifth",
			path:  "five",
			kind:  "ReplicaSet",
			hooks: []release.Hook_Event{release.Hook_POST_DELETE, release.Hook_POST_INSTALL},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: fifth
  annotations:
    "helm.sh/hook": post-delete, post-install
`,
		}, {
			// Regression test: files with an underscore in the base name should be skipped.
			name:     "sixth",
			path:     "six/_six",
			kind:     "ReplicaSet",
			hooks:    []release.Hook_Event{},
			manifest: `invalid manifest`, // This will fail if partial is not skipped.
		}, {
			// Regression test: files with no content should be skipped.
			name:     "seventh",
			path:     "seven",
			kind:     "ReplicaSet",
			hooks:    []release.Hook_Event{},
			manifest: "",
		},
	}

	manifests := make(map[string]string, len(data))
	for _, o := range data {
		manifests[o.path] = o.manifest
	}

	hs, generic, err := sortManifests(manifests, chartutil.NewVersionSet("v1", "v1beta1"), InstallOrder)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// This test will fail if 'six' or 'seven' was added.
	if len(generic) != 1 {
		t.Errorf("Expected 1 generic manifest, got %d", len(generic))
	}

	if len(hs) != 3 {
		t.Errorf("Expected 3 hooks, got %d", len(hs))
	}

	for _, out := range hs {
		found := false
		for _, expect := range data {
			if out.Path == expect.path {
				found = true
				if out.Path != expect.path {
					t.Errorf("Expected path %s, got %s", expect.path, out.Path)
				}
				if out.Name != expect.name {
					t.Errorf("Expected name %s, got %s", expect.name, out.Name)
				}
				if out.Kind != expect.kind {
					t.Errorf("Expected kind %s, got %s", expect.kind, out.Kind)
				}
				for i := 0; i < len(out.Events); i++ {
					if out.Events[i] != expect.hooks[i] {
						t.Errorf("Expected event %d, got %d", expect.hooks[i], out.Events[i])
					}
				}
			}
		}
		if !found {
			t.Errorf("Result not found: %v", out)
		}
	}

	// Verify the sort order
	sorted := make([]manifest, len(data))
	for i, s := range data {
		var sh util.SimpleHead
		err := yaml.Unmarshal([]byte(s.manifest), &sh)
		if err != nil {
			// This is expected for manifests that are corrupt or empty.
			t.Log(err)
		}
		sorted[i] = manifest{
			content: s.manifest,
			name:    s.name,
			head:    &sh,
		}
	}
	sorted = sortByKind(sorted, InstallOrder)
	for i, m := range generic {
		if m.content != sorted[i].content {
			t.Errorf("Expected %q, got %q", m.content, sorted[i].content)
		}
	}

}

func TestVersionSet(t *testing.T) {
	vs := chartutil.NewVersionSet("v1", "v1beta1", "extensions/alpha5", "batch/v1")

	if l := len(vs); l != 4 {
		t.Errorf("Expected 4, got %d", l)
	}

	if !vs.Has("extensions/alpha5") {
		t.Error("No match for alpha5")
	}

	if vs.Has("nosuch/extension") {
		t.Error("Found nonexistent extension")
	}
}
