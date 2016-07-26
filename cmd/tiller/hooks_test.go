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

package main

import (
	"testing"

	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestSortHooks(t *testing.T) {

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
    doesnt: matter
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

	hs, generic, err := sortHooks(manifests)
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

}
