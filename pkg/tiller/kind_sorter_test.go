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
			name: "i",
			head: &util.SimpleHead{Kind: "ClusterRole"},
		},
		{
			name: "j",
			head: &util.SimpleHead{Kind: "ClusterRoleBinding"},
		},
		{
			name: "e",
			head: &util.SimpleHead{Kind: "ConfigMap"},
		},
		{
			name: "u",
			head: &util.SimpleHead{Kind: "CronJob"},
		},
		{
			name: "n",
			head: &util.SimpleHead{Kind: "DaemonSet"},
		},
		{
			name: "r",
			head: &util.SimpleHead{Kind: "Deployment"},
		},
		{
			name: "!",
			head: &util.SimpleHead{Kind: "HonkyTonkSet"},
		},
		{
			name: "v",
			head: &util.SimpleHead{Kind: "Ingress"},
		},
		{
			name: "t",
			head: &util.SimpleHead{Kind: "Job"},
		},
		{
			name: "c",
			head: &util.SimpleHead{Kind: "LimitRange"},
		},
		{
			name: "a",
			head: &util.SimpleHead{Kind: "Namespace"},
		},
		{
			name: "f",
			head: &util.SimpleHead{Kind: "PersistentVolume"},
		},
		{
			name: "g",
			head: &util.SimpleHead{Kind: "PersistentVolumeClaim"},
		},
		{
			name: "o",
			head: &util.SimpleHead{Kind: "Pod"},
		},
		{
			name: "q",
			head: &util.SimpleHead{Kind: "ReplicaSet"},
		},
		{
			name: "p",
			head: &util.SimpleHead{Kind: "ReplicationController"},
		},
		{
			name: "b",
			head: &util.SimpleHead{Kind: "ResourceQuota"},
		},
		{
			name: "k",
			head: &util.SimpleHead{Kind: "Role"},
		},
		{
			name: "l",
			head: &util.SimpleHead{Kind: "RoleBinding"},
		},
		{
			name: "d",
			head: &util.SimpleHead{Kind: "Secret"},
		},
		{
			name: "m",
			head: &util.SimpleHead{Kind: "Service"},
		},
		{
			name: "h",
			head: &util.SimpleHead{Kind: "ServiceAccount"},
		},
		{
			name: "s",
			head: &util.SimpleHead{Kind: "StatefulSet"},
		},
		{
			name:    "w",
			content: "",
			head:    &util.SimpleHead{Kind: "APIService"},
		},
	}

	for _, test := range []struct {
		description string
		order       SortOrder
		expected    string
	}{
		{"install", InstallOrder, "abcdefghijklmnopqrstuvw!"},
		{"uninstall", UninstallOrder, "wvmutsrqponlkjihgfedcba!"},
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
