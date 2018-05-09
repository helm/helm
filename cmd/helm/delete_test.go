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

	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

func TestDelete(t *testing.T) {

	rels := []*release.Release{helm.ReleaseMock(&helm.MockReleaseOptions{Name: "aeneas"})}

	tests := []releaseCase{
		{
			name:   "basic delete",
			cmd:    "delete aeneas",
			golden: "output/delete.txt",
			rels:   rels,
		},
		{
			name:   "delete with timeout",
			cmd:    "delete aeneas --timeout 120",
			golden: "output/delete-timeout.txt",
			rels:   rels,
		},
		{
			name:   "delete without hooks",
			cmd:    "delete aeneas --no-hooks",
			golden: "output/delete-no-hooks.txt",
			rels:   rels,
		},
		{
			name:   "purge",
			cmd:    "delete aeneas --purge",
			golden: "output/delete-purge.txt",
			rels:   rels,
		},
		{
			name:      "delete without release",
			cmd:       "delete",
			golden:    "output/delete-no-args.txt",
			wantError: true,
		},
	}
	testReleaseCmd(t, tests)
}
