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
	"bytes"
	"strings"
	"testing"
)

func TestDependencyListCmd(t *testing.T) {

	tests := []struct {
		name   string
		args   []string
		expect string
		err    bool
	}{
		{
			name: "No such chart",
			args: []string{"/no/such/chart"},
			err:  true,
		},
		{
			name:   "No requirements.yaml",
			args:   []string{"testdata/testcharts/alpine"},
			expect: "WARNING: no requirements at ",
		},
		{
			name: "Requirements in chart dir",
			args: []string{"testdata/testcharts/reqtest"},
			expect: "NAME        \tVERSION\tREPOSITORY                \tSTATUS  \n" +
				"reqsubchart \t0.1.0  \thttps://example.com/charts\tunpacked\n" +
				"reqsubchart2\t0.2.0  \thttps://example.com/charts\tunpacked\n" +
				"reqsubchart3\t>=0.1.0\thttps://example.com/charts\tok      \n\n",
		},
		{
			name:   "Requirements in chart archive",
			args:   []string{"testdata/testcharts/reqtest-0.1.0.tgz"},
			expect: "NAME        \tVERSION\tREPOSITORY                \tSTATUS \nreqsubchart \t0.1.0  \thttps://example.com/charts\tmissing\nreqsubchart2\t0.2.0  \thttps://example.com/charts\tmissing\n",
		},
	}

	for _, tt := range tests {
		buf := bytes.NewBuffer(nil)
		dlc := newDependencyListCmd(buf)
		if err := dlc.RunE(dlc, tt.args); err != nil {
			if tt.err {
				continue
			}
			t.Errorf("Test %q: %s", tt.name, err)
			continue
		}

		got := buf.String()
		if !strings.Contains(got, tt.expect) {
			t.Errorf("Test: %q, Expected:\n%q\nGot:\n%q", tt.name, tt.expect, got)
		}
	}

}
