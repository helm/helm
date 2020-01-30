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

package releaseutil

import (
	"reflect"
	"testing"

	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
)

func TestSortManifests(t *testing.T) {

	data := []struct {
		name     []string
		path     string
		kind     []string
		hooks    map[string][]release.HookEvent
		manifest string
	}{
		{
			name:  []string{"first"},
			path:  "one",
			kind:  []string{"Job"},
			hooks: map[string][]release.HookEvent{"first": {release.HookPreInstall}},
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
			name:  []string{"second"},
			path:  "two",
			kind:  []string{"ReplicaSet"},
			hooks: map[string][]release.HookEvent{"second": {release.HookPostInstall}},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: second
  annotations:
    "helm.sh/hook": post-install
`,
		}, {
			name:  []string{"third"},
			path:  "three",
			kind:  []string{"ReplicaSet"},
			hooks: map[string][]release.HookEvent{"third": nil},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: third
  annotations:
    "helm.sh/hook": no-such-hook
`,
		}, {
			name:  []string{"fourth"},
			path:  "four",
			kind:  []string{"Pod"},
			hooks: map[string][]release.HookEvent{"fourth": nil},
			manifest: `kind: Pod
apiVersion: v1
metadata:
  name: fourth
  annotations:
    nothing: here`,
		}, {
			name:  []string{"fifth"},
			path:  "five",
			kind:  []string{"ReplicaSet"},
			hooks: map[string][]release.HookEvent{"fifth": {release.HookPostDelete, release.HookPostInstall}},
			manifest: `kind: ReplicaSet
apiVersion: v1beta1
metadata:
  name: fifth
  annotations:
    "helm.sh/hook": post-delete, post-install
`,
		}, {
			// Regression test: files with an underscore in the base name should be skipped.
			name:     []string{"sixth"},
			path:     "six/_six",
			kind:     []string{"ReplicaSet"},
			hooks:    map[string][]release.HookEvent{"sixth": nil},
			manifest: `invalid manifest`, // This will fail if partial is not skipped.
		}, {
			// Regression test: files with no content should be skipped.
			name:     []string{"seventh"},
			path:     "seven",
			kind:     []string{"ReplicaSet"},
			hooks:    map[string][]release.HookEvent{"seventh": nil},
			manifest: "",
		},
		{
			name:  []string{"eighth", "example-test"},
			path:  "eight",
			kind:  []string{"ConfigMap", "Pod"},
			hooks: map[string][]release.HookEvent{"eighth": nil, "example-test": {release.HookTest}},
			manifest: `kind: ConfigMap
apiVersion: v1
metadata:
  name: eighth
data:
  name: value
---
apiVersion: v1
kind: Pod
metadata:
  name: example-test
  annotations:
    "helm.sh/hook": test
`,
		},
	}

	manifests := make(map[string]string, len(data))
	for _, o := range data {
		manifests[o.path] = o.manifest
	}

	hs, generic, err := SortManifests(manifests, chartutil.VersionSet{"v1", "v1beta1"}, InstallOrder)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	// This test will fail if 'six' or 'seven' was added.
	if len(generic) != 2 {
		t.Errorf("Expected 2 generic manifests, got %d", len(generic))
	}

	if len(hs) != 4 {
		t.Errorf("Expected 4 hooks, got %d", len(hs))
	}

	for _, out := range hs {
		found := false
		for _, expect := range data {
			if out.Path == expect.path {
				found = true
				if out.Path != expect.path {
					t.Errorf("Expected path %s, got %s", expect.path, out.Path)
				}
				nameFound := false
				for _, expectedName := range expect.name {
					if out.Name == expectedName {
						nameFound = true
					}
				}
				if !nameFound {
					t.Errorf("Got unexpected name %s", out.Name)
				}
				kindFound := false
				for _, expectedKind := range expect.kind {
					if out.Kind == expectedKind {
						kindFound = true
					}
				}
				if !kindFound {
					t.Errorf("Got unexpected kind %s", out.Kind)
				}

				expectedHooks := expect.hooks[out.Name]
				if !reflect.DeepEqual(expectedHooks, out.Events) {
					t.Errorf("expected events: %v but got: %v", expectedHooks, out.Events)
				}

			}
		}
		if !found {
			t.Errorf("Result not found: %v", out)
		}
	}

	// Verify the sort order
	sorted := []Manifest{}
	for _, s := range data {
		manifests := SplitManifests(s.manifest)

		for _, m := range manifests {
			var sh SimpleHead
			if err := yaml.Unmarshal([]byte(m), &sh); err != nil {
				// This is expected for manifests that are corrupt or empty.
				t.Log(err)
				continue
			}

			name := sh.Metadata.Name

			// only keep track of non-hook manifests
			if s.hooks[name] == nil {
				another := Manifest{
					Content: m,
					Name:    name,
					Head:    &sh,
				}
				sorted = append(sorted, another)
			}
		}
	}

	sorted = sortManifestsByKind(sorted, InstallOrder)
	for i, m := range generic {
		if m.Content != sorted[i].Content {
			t.Errorf("Expected %q, got %q", m.Content, sorted[i].Content)
		}
	}
}
