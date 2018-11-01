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

package main

import (
	"io"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestGetValuesCmd(t *testing.T) {
	releaseWithValues := helm.ReleaseMock(&helm.MockReleaseOptions{
		Name:   "thomas-guide",
		Chart:  &chart.Chart{Values: &chart.Config{Raw: `foo2: "bar2"`}},
		Config: &chart.Config{Raw: `foo: "bar"`},
	})

	tests := []releaseCase{
		{
			name:     "get values with a release",
			resp:     helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"}),
			args:     []string{"thomas-guide"},
			expected: "name: value",
			rels:     []*release.Release{helm.ReleaseMock(&helm.MockReleaseOptions{Name: "thomas-guide"})},
		},
		{
			name:     "get values with json format",
			resp:     releaseWithValues,
			args:     []string{"thomas-guide"},
			flags:    []string{"--output", "json"},
			expected: "{\"foo\":\"bar\"}",
			rels:     []*release.Release{releaseWithValues},
		},
		{
			name:     "get all values with json format",
			resp:     releaseWithValues,
			args:     []string{"thomas-guide"},
			flags:    []string{"--all", "--output", "json"},
			expected: "{\"foo\":\"bar\",\"foo2\":\"bar2\"}",
			rels:     []*release.Release{releaseWithValues},
		},
		{
			name: "get values requires release name arg",
			err:  true,
		},
		{
			name:  "get values with invalid output format",
			resp:  releaseWithValues,
			args:  []string{"thomas-guide"},
			flags: []string{"--output", "INVALID_FORMAT"},
			rels:  []*release.Release{releaseWithValues},
			err:   true,
		},
	}
	cmd := func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newGetValuesCmd(c, out)
	}
	runReleaseCases(t, tests, cmd)
}
