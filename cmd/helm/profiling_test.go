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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parsePProfPaths(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	tests := []struct {
		name string
		env  string
		want map[string]string
	}{
		{
			name: "no env",
			env:  "",
			want: map[string]string{},
		},
		{
			name: "single path",
			env:  "cpu=cpu.pprof",
			want: map[string]string{
				"cpu": cwd + "/cpu.pprof",
			},
		},
		{
			name: "mem and cpu paths",
			env:  "cpu=cpu.pprof,mem=mem.pprof",
			want: map[string]string{
				"cpu": cwd + "/cpu.pprof",
				"mem": cwd + "/mem.pprof",
			},
		},
		{
			name: "extra commas",
			env:  "cpu=cpu.pprof,mem=mem.pprof,",
			want: map[string]string{
				"cpu": cwd + "/cpu.pprof",
				"mem": cwd + "/mem.pprof",
			},
		},
		{
			name: "unknown keys",
			env:  "cpu=cpu.pprof,mem=mem.pprof,foo=bar",
			want: map[string]string{
				"cpu": cwd + "/cpu.pprof",
				"mem": cwd + "/mem.pprof",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePProfPaths(tt.env)
			assert.Equalf(t, tt.want, got, "parsePProfPaths() = %v, want %v", got, tt.want)
		})
	}
}
