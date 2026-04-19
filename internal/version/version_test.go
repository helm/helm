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

package version

import "testing"

func TestIsGoTestBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "go test temp binary", path: "/tmp/go-build1234/b001/version.test", want: true},
		{name: "go test windows binary", path: `C:\Temp\go-build1234\b001\version.test.exe`, want: true},
		{name: "regular binary", path: "/usr/local/bin/helm", want: false},
		{name: "user binary with test in name only", path: "/usr/local/bin/helm-test", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isGoTestBinary(tt.path); got != tt.want {
				t.Fatalf("isGoTestBinary(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestGetUsesStableKubeClientVersionInTestBinary(t *testing.T) {
	t.Parallel()

	if got := Get().KubeClientVersion; got != kubeClientGoVersionTesting {
		t.Fatalf("Get().KubeClientVersion = %q, want %q", got, kubeClientGoVersionTesting)
	}
}
