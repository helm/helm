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
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInstall(t *testing.T) {
	tests := []releaseCase{
		// Install, base case
		{
			name:     "basic install",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--name aeneas", " "),
			expected: "aeneas",
			resp:     releaseMock("aeneas"),
		},
		// Install, no hooks
		{
			name:     "install without hooks",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--name aeneas --no-hooks", " "),
			expected: "juno",
			resp:     releaseMock("juno"),
		},
		// Install, no charts
		{
			name: "install with no chart specified",
			args: []string{},
			err:  true,
		},
	}

	runReleaseCases(t, tests, func(c *fakeReleaseClient, out io.Writer) *cobra.Command {
		return newInstallCmd(c, out)
	})
}
