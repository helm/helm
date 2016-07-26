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
			resp:     releaseMock(&releaseOptions{name: "aeneas"}),
		},
		// Install, no hooks
		{
			name:     "install without hooks",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--name aeneas --no-hooks", " "),
			expected: "juno",
			resp:     releaseMock(&releaseOptions{name: "juno"}),
		},
		// Install, values from cli
		{
			name:     "install with values",
			args:     []string{"testdata/testcharts/alpine"},
			flags:    strings.Split("--set foo=bar", " "),
			resp:     releaseMock(&releaseOptions{name: "virgil"}),
			expected: "virgil",
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

func TestValues(t *testing.T) {
	args := "sailor=sinbad,good,port.source=baghdad,port.destination=basrah"
	vobj := new(values)
	vobj.Set(args)

	if vobj.Type() != "struct" {
		t.Fatalf("Expected Type to be struct, got %s", vobj.Type())
	}

	vals := vobj.pairs
	if fmt.Sprint(vals["good"]) != "true" {
		t.Errorf("Expected good to be true. Got %v", vals["good"])
	}

	port := vals["port"].(map[string]interface{})

	if fmt.Sprint(port["source"]) != "baghdad" {
		t.Errorf("Expected source to be baghdad. Got %s", port["source"])
	}
	if fmt.Sprint(port["destination"]) != "basrah" {
		t.Errorf("Expected source to be baghdad. Got %s", port["source"])
	}

	y := `good: true
port:
  destination: basrah
  source: baghdad
sailor: sinbad
`
	out, err := vobj.yaml()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != y {
		t.Errorf("Expected YAML to be \n%s\nGot\n%s\n", y, out)
	}

	if vobj.String() != y {
		t.Errorf("Expected String() to be \n%s\nGot\n%s\n", y, out)
	}
}
