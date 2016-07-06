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
		},
	}

	manifests := make(map[string]string, len(data))
	for _, o := range data {
		manifests[o.path] = o.manifest
	}

	hs, generic := sortHooks(manifests)

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
