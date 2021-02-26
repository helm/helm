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
package chart

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		md  *Metadata
		err error
	}{
		{
			nil,
			ValidationError("chart.metadata is required"),
		},
		{
			&Metadata{Name: "test", Version: "1.0"},
			ValidationError("chart.metadata.apiVersion is required"),
		},
		{
			&Metadata{APIVersion: "v2", Version: "1.0"},
			ValidationError("chart.metadata.name is required"),
		},
		{
			&Metadata{Name: "test", APIVersion: "v2"},
			ValidationError("chart.metadata.version is required"),
		},
		{
			&Metadata{Name: "test", APIVersion: "v2", Version: "1.0", Type: "test"},
			ValidationError("chart.metadata.type must be application or library"),
		},
		{
			&Metadata{Name: "test", APIVersion: "v2", Version: "1.0", Type: "application"},
			nil,
		},
		{
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Dependencies: []*Dependency{
					{Name: "dependency", Alias: "legal-alias"},
				},
			},
			nil,
		},
		{
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Dependencies: []*Dependency{
					{Name: "bad", Alias: "illegal alias"},
				},
			},
			ValidationError("dependency \"bad\" has disallowed characters in the alias"),
		},
		{
			&Metadata{APIVersion: "v2", Name: "test", Version: "1.2.3.4"},
			ValidationError("chart.metadata.version \"1.2.3.4\" is invalid"),
		},
	}

	for _, tt := range tests {
		result := tt.md.Validate()
		if result != tt.err {
			t.Errorf("expected '%s', got '%s'", tt.err, result)
		}
	}
}

func TestValidate_sanitize(t *testing.T) {
	md := &Metadata{APIVersion: "v2", Name: "test", Version: "1.0", Description: "\adescr\u0081iption\rtest", Maintainers: []*Maintainer{{Name: "\r"}}}
	if err := md.Validate(); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if md.Description != "description test" {
		t.Fatalf("description was not sanitized: %q", md.Description)
	}
	if md.Maintainers[0].Name != " " {
		t.Fatal("maintainer name was not sanitized")
	}
}
