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
	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestGetVersion(t *testing.T) {
	tests := []releaseCase{
		{
			name:     "get chart version of release",
			args:     []string{"kodiak"},
			expected: "0.1.0-beta.1",
			resp:     helm.ReleaseMock(&helm.MockReleaseOptions{Name: "kodiak"}),
			rels:     []*release.Release{helm.ReleaseMock(&helm.MockReleaseOptions{Name: "kodiak"})},
		},
		{
			name: "get version without args",
			args: []string{},
			err:  true,
		},
	}
	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newGetVersionCmd(c, out)
	})
}
