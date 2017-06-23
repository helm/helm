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
	"io"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
)

func TestDelete(t *testing.T) {

	tests := []releaseCase{
		{
			name:     "basic delete",
			args:     []string{"aeneas"},
			flags:    []string{},
			expected: "", // Output of a delete is an empty string and exit 0.
			resp:     releaseMock(&releaseOptions{name: "aeneas"}),
		},
		{
			name:     "delete with timeout",
			args:     []string{"aeneas"},
			flags:    []string{"--timeout", "120"},
			expected: "",
			resp:     releaseMock(&releaseOptions{name: "aeneas"}),
		},
		{
			name:     "delete without hooks",
			args:     []string{"aeneas"},
			flags:    []string{"--no-hooks"},
			expected: "",
			resp:     releaseMock(&releaseOptions{name: "aeneas"}),
		},
		{
			name:     "purge",
			args:     []string{"aeneas"},
			flags:    []string{"--purge"},
			expected: "",
			resp:     releaseMock(&releaseOptions{name: "aeneas"}),
		},
		{
			name: "delete without release",
			args: []string{},
			err:  true,
		},
	}
	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newDeleteCmd(c, out)
	})
}
