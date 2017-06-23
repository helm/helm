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

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/version"
)

func TestVersion(t *testing.T) {

	lver := version.GetVersionProto().SemVer
	sver := "1.2.3-fakeclient+testonly"

	tests := []struct {
		name           string
		client, server bool
		args           []string
		fail           bool
	}{
		{"default", true, true, []string{}, false},
		{"client", true, false, []string{"-c"}, false},
		{"server", false, true, []string{"-s"}, false},
	}

	settings.TillerHost = "fake-localhost"
	for _, tt := range tests {
		b := new(bytes.Buffer)
		c := &helm.FakeClient{}

		cmd := newVersionCmd(c, b)
		cmd.ParseFlags(tt.args)
		if err := cmd.RunE(cmd, tt.args); err != nil {
			if tt.fail {
				continue
			}
			t.Fatal(err)
		}

		if tt.client && !strings.Contains(b.String(), lver) {
			t.Errorf("Expected %q to contain %q", b.String(), lver)
		}
		if tt.server && !strings.Contains(b.String(), sver) {
			t.Errorf("Expected %q to contain %q", b.String(), sver)
		}
	}
}
