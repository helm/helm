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

package registry

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

const testAliasesFile = "testdata/aliases.yaml"

func TestAliasesFile(t *testing.T) {
	a := NewAliasesFile()
	a.SetAlias("staging", "oci://example.com/charts/staging")
	a.SetAlias("production", "oci://example.com/charts/production")

	a.SetSubstitution("oci://example.com/charts/production", "oci://example.com/qa-environment/charts/production")

	if len(a.Aliases) != 2 {
		t.Fatal("Expected 2 aliases")
	}

	if len(a.Substitutions) != 1 {
		t.Fatal("Expected 1 substitution")
	}

	if !a.RemoveAlias("staging") {
		t.Fatal("Expected staging alias to exist")
	}

	if a.RemoveAlias("staging") {
		t.Fatal("Expected staging alias to not exist")
	}

	if len(a.Aliases) != 1 {
		t.Fatal("Expected 1 alias")
	}

	if !a.RemoveSubstitution("oci://example.com/charts/production") {
		t.Fatal("Expected 'oci://example.com/charts/production' substitution to exist")
	}

	if a.RemoveSubstitution("oci://example.com/charts/production") {
		t.Fatal("Expected 'oci://example.com/charts/production' substitution to not exist")
	}

	if len(a.Substitutions) != 0 {
		t.Fatal("Expected 0 substitutions")
	}
}

func TestNewAliasesFile(t *testing.T) {
	expects := NewAliasesFile()
	expects.SetAlias("staging", "oci://example.com/charts/staging")
	expects.SetAlias("production", "oci://example.com/charts/production")
	expects.SetAlias("dev", "oci://example.com/charts/dev")
	expects.SetSubstitution("oci://example.com/charts/dev", "oci://dev.example.com/charts")
	expects.SetSubstitution("oci://example.com/charts/staging", "oci://staging.example.com/charts")
	expects.SetSubstitution("https://example.com/stable/charts", "oci://stable.example.com/charts")

	file, err := LoadAliasesFile(testAliasesFile)
	if err != nil {
		t.Errorf("%q could not be loaded: %s", testAliasesFile, err)
	}

	if !reflect.DeepEqual(expects.APIVersion, file.APIVersion) {
		t.Fatalf("Unexpected apiVersion: %#v", file.APIVersion)
	}

	if !reflect.DeepEqual(expects.Aliases, file.Aliases) {
		t.Fatalf("Unexpected aliases: %#v", file.Aliases)
	}

	if !reflect.DeepEqual(expects.Substitutions, file.Substitutions) {
		t.Fatalf("Unexpected substitutions: %#v", file.Substitutions)
	}
}

func TestWriteAliasesFile(t *testing.T) {
	expects := NewAliasesFile()
	expects.SetAlias("dev", "oci://example.com/charts/dev")
	expects.SetSubstitution("oci://example.com/charts/dev", "oci://dev.example.com/charts")

	file, err := os.CreateTemp("", "helm-aliases")
	if err != nil {
		t.Errorf("failed to create test-file (%v)", err)
	}
	defer os.Remove(file.Name())
	if err := expects.WriteAliasesFile(file.Name(), 0o644); err != nil {
		t.Errorf("failed to write file (%v)", err)
	}

	aliases, err := LoadAliasesFile(file.Name())
	if err != nil {
		t.Errorf("failed to load file (%v)", err)
	}

	if !reflect.DeepEqual(expects, aliases) {
		t.Errorf("aliases inconsistent after saving and reloading:\nexpected: %#v\nactual: %#v", expects, aliases)
	}
}

func TestAliasNotExists(t *testing.T) {
	if _, err := LoadAliasesFile("/this/path/does/not/exist.yaml"); err == nil {
		t.Errorf("expected err to be non-nil when path does not exist")
	} else if !strings.Contains(err.Error(), "couldn't load aliases file") {
		t.Errorf("expected prompt `couldn't load aliases file`")
	}
}

func TestAliases_performSubstitutions(t *testing.T) {
	substitutions := NewAliasesFile()
	substitutions.SetSubstitution("oci://example.com/charts", "oci://example.com/charts/dev")
	substitutions.SetSubstitution("oci://length.example.com", "oci://shorter.length.example.com")
	substitutions.SetSubstitution("oci://length.example.com/charts", "oci://longer.length.example.com/charts")
	substitutions.SetSubstitution("oci://multiple.example.com", "oci://example.com/charts")
	substitutions.SetSubstitution("oci://localhost:5000/", "oci://staging.example.com/charts/")
	substitutions.SetSubstitution("https://example.com/vendor", "oci://vendor.example.com/charts/")
	substitutions.SetSubstitution("oci://one.example.com", "oci://two.example.com")
	substitutions.SetSubstitution("oci://two.example.com", "oci://one.example.com")

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "basicOCIReplacement",
			source: "oci://localhost:5000/myrepo",
			want:   "oci://staging.example.com/charts/myrepo",
		},
		{
			name:   "exacltyAsRequested",
			source: "https://example.com/vendor-dev/some-chart-repo",
			want:   "oci://vendor.example.com/charts/-dev/some-chart-repo",
		},
		{
			name:   "multipleReplacements",
			source: "oci://multiple.example.com/myrepo",
			want:   "oci://example.com/charts/dev/myrepo",
		},
		{
			name:   "norecursion",
			source: "oci://one.example.com/myrepo",
			want:   "oci://one.example.com/myrepo",
		},
		{
			name:   "usedOnlyOnce",
			source: "oci://example.com/charts/myrepo",
			want:   "oci://example.com/charts/dev/myrepo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := substitutions.performSubstitutions(tt.source); got != tt.want {
				t.Errorf("Aliases.performSubstitutions() = %v, want %v", got, tt.want)
			}
		})
	}
}
