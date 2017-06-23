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

func TestRollbackCmd(t *testing.T) {

	tests := []releaseCase{
		{
			name:     "rollback a release",
			args:     []string{"funny-honey", "1"},
			expected: "Rollback was a success! Happy Helming!",
		},
		{
			name:     "rollback a release with timeout",
			args:     []string{"funny-honey", "1"},
			flags:    []string{"--timeout", "120"},
			expected: "Rollback was a success! Happy Helming!",
		},
		{
			name:     "rollback a release with wait",
			args:     []string{"funny-honey", "1"},
			flags:    []string{"--wait"},
			expected: "Rollback was a success! Happy Helming!",
		},
		{
			name: "rollback a release without revision",
			args: []string{"funny-honey"},
			err:  true,
		},
	}

	cmd := func(c *helm.FakeClient, out io.Writer) *cobra.Command {
		return newRollbackCmd(c, out)
	}

	runReleaseCases(t, tests, cmd)

}
