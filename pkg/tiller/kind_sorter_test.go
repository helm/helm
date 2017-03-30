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
	"bytes"
	"testing"

	util "k8s.io/helm/pkg/releaseutil"
)

func TestKindSorter(t *testing.T) {
	manifests := []manifest{
		{
			name:    "m",
			content: "",
			head:    &util.SimpleHead{Kind: "ClusterRole"},
		},
		{
			name:    " ",
			content: "",
			head:    &util.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			name:    "e",
			content: "",
			head:    &util.SimpleHead{Kind: "ConfigMap"},
		},
		{
			name:    "k",
			content: "",
			head:    &util.SimpleHead{Kind: "Deployment"},
		},
		{
			name:    "!",
			content: "",
			head:    &util.SimpleHead{Kind: "HonkyTonkSet"},
		},
		{
			name:    "s",
			content: "",
			head:    &util.SimpleHead{Kind: "Job"},
		},
		{
			name:    "h",
			content: "",
			head:    &util.SimpleHead{Kind: "Namespace"},
		},
		{
			name:    "w",
			content: "",
			head:    &util.SimpleHead{Kind: "Role"},
		},
		{
			name:    "o",
			content: "",
			head:    &util.SimpleHead{Kind: "RoleBinding"},
		},
		{
			name:    "r",
			content: "",
			head:    &util.SimpleHead{Kind: "Service"},
		},
		{
			name:    "l",
			content: "",
			head:    &util.SimpleHead{Kind: "ServiceAccount"},
		},
	}

	for _, test := range []struct {
		description string
		order       SortOrder
		expected    string
	}{
		{"install", InstallOrder, "helm works!"},
		{"uninstall", UninstallOrder, "rkeow mlsh!"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			defer buf.Reset()
			for _, r := range sortByKind(manifests, test.order) {
				buf.WriteString(r.name)
			}
			if got := buf.String(); got != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, got)
			}
		})
	}
}
