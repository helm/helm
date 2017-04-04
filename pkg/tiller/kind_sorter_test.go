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
			name:    "i",
			content: "",
			head:    &util.SimpleHead{Kind: "ClusterRole"},
		},
		{
			name:    "j",
			content: "",
			head:    &util.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			name:    "e",
			content: "",
			head:    &util.SimpleHead{Kind: "ConfigMap"},
		},
		{
			name:    "u",
			content: "",
			head:    &util.SimpleHead{Kind: "CronJob"},
		},
		{
			name:    "n",
			content: "",
			head:    &util.SimpleHead{Kind: "DaemonSet"},
		},
		{
			name:    "r",
			content: "",
			head:    &util.SimpleHead{Kind: "Deployment"},
		},
		{
			name:    "!",
			content: "",
			head:    &util.SimpleHead{Kind: "HonkyTonkSet"},
		},
		{
			name:    "v",
			content: "",
			head:    &util.SimpleHead{Kind: "Ingress"},
		},
		{
			name:    "t",
			content: "",
			head:    &util.SimpleHead{Kind: "Job"},
		},
		{
			name:    "c",
			content: "",
			head:    &util.SimpleHead{Kind: "LimitRange"},
		},
		{
			name:    "a",
			content: "",
			head:    &util.SimpleHead{Kind: "Namespace"},
		},
		{
			name:    "f",
			content: "",
			head:    &util.SimpleHead{Kind: "PersistentVolume"},
		},
		{
			name:    "g",
			content: "",
			head:    &util.SimpleHead{Kind: "PersistentVolumeClaim"},
		},
		{
			name:    "o",
			content: "",
			head:    &util.SimpleHead{Kind: "Pod"},
		},
		{
			name:    "q",
			content: "",
			head:    &util.SimpleHead{Kind: "ReplicaSet"},
		},
		{
			name:    "p",
			content: "",
			head:    &util.SimpleHead{Kind: "ReplicationController"},
		},
		{
			name:    "b",
			content: "",
			head:    &util.SimpleHead{Kind: "ResourceQuota"},
		},
		{
			name:    "k",
			content: "",
			head:    &util.SimpleHead{Kind: "Role"},
		},
		{
			name:    "l",
			content: "",
			head:    &util.SimpleHead{Kind: "RoleBinding"},
		},
		{
			name:    "d",
			content: "",
			head:    &util.SimpleHead{Kind: "Secret"},
		},
		{
			name:    "m",
			content: "",
			head:    &util.SimpleHead{Kind: "Service"},
		},
		{
			name:    "h",
			content: "",
			head:    &util.SimpleHead{Kind: "ServiceAccount"},
		},
		{
			name:    "s",
			content: "",
			head:    &util.SimpleHead{Kind: "StatefulSet"},
		},
	}

	for _, test := range []struct {
		description string
		order       SortOrder
		expected    string
	}{
		{"install", InstallOrder, "abcdefghijklmnopqrstuv!"},
		{"uninstall", UninstallOrder, "vmutsrqponlkjihgfedcba!"},
	} {
		var buf bytes.Buffer
		t.Run(test.description, func(t *testing.T) {
			if got, want := len(test.expected), len(manifests); got != want {
				t.Fatalf("Expected %d names in order, got %d", want, got)
			}
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
