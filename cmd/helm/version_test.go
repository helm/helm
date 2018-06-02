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
	"fmt"
	"io"
	"regexp"
	"testing"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/version"
)

func TestVersion(t *testing.T) {
	lver := regexp.QuoteMeta(version.GetVersionProto().SemVer)
	sver := regexp.QuoteMeta("1.2.3-fakeclient+testonly")
	clientVersion := fmt.Sprintf("Client: &version\\.Version{SemVer:\"%s\", GitCommit:\"\", GitTreeState:\"\"}\n", lver)
	serverVersion := fmt.Sprintf("Server: &version\\.Version{SemVer:\"%s\", GitCommit:\"\", GitTreeState:\"\"}\n", sver)

	tests := []releaseCase{
		{
			name:     "default",
			args:     []string{},
			expected: clientVersion + serverVersion,
		},
		{
			name:     "client",
			args:     []string{},
			flags:    []string{"-c"},
			expected: clientVersion,
		},
		{
			name:     "server",
			args:     []string{},
			flags:    []string{"-s"},
			expected: serverVersion,
		},
		{
			name:     "template",
			args:     []string{},
			flags:    []string{"--template", "{{ .Client.SemVer }} {{ .Server.SemVer }}"},
			expected: lver + " " + sver,
		},
	}
	settings.TillerHost = "fake-localhost"
	runReleaseCases(t, tests, func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newVersionCmd(c, out)
	})
}
