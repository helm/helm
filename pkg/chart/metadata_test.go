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
		name string
		md   *Metadata
		err  error
	}{
		{
			"chart without metadata",
			nil,
			ValidationError("chart.metadata is required"),
		},
		{
			"chart without apiVersion",
			&Metadata{Name: "test", Version: "1.0"},
			ValidationError("chart.metadata.apiVersion is required"),
		},
		{
			"chart without name",
			&Metadata{APIVersion: "v2", Version: "1.0"},
			ValidationError("chart.metadata.name is required"),
		},
		{
			"chart without name",
			&Metadata{Name: "../../test", APIVersion: "v2", Version: "1.0"},
			ValidationError("chart.metadata.name \"../../test\" is invalid"),
		},
		{
			"chart without version",
			&Metadata{Name: "test", APIVersion: "v2"},
			ValidationError("chart.metadata.version is required"),
		},
		{
			"chart with bad type",
			&Metadata{Name: "test", APIVersion: "v2", Version: "1.0", Type: "test"},
			ValidationError("chart.metadata.type must be application or library"),
		},
		{
			"chart without dependency",
			&Metadata{Name: "test", APIVersion: "v2", Version: "1.0", Type: "application"},
			nil,
		},
		{
			"dependency with valid alias",
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
			"dependency with bad characters in alias",
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
			"same dependency twice",
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Dependencies: []*Dependency{
					{Name: "foo", Alias: ""},
					{Name: "foo", Alias: ""},
				},
			},
			ValidationError("more than one dependency with name or alias \"foo\""),
		},
		{
			"two dependencies with alias from second dependency shadowing first one",
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Dependencies: []*Dependency{
					{Name: "foo", Alias: ""},
					{Name: "bar", Alias: "foo"},
				},
			},
			ValidationError("more than one dependency with name or alias \"foo\""),
		},
		{
			// this case would make sense and could work in future versions of Helm, currently template rendering would
			// result in undefined behaviour
			"same dependency twice with different version",
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Dependencies: []*Dependency{
					{Name: "foo", Alias: "", Version: "1.2.3"},
					{Name: "foo", Alias: "", Version: "1.0.0"},
				},
			},
			ValidationError("more than one dependency with name or alias \"foo\""),
		},
		{
			// this case would make sense and could work in future versions of Helm, currently template rendering would
			// result in undefined behaviour
			"two dependencies with same name but different repos",
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Dependencies: []*Dependency{
					{Name: "foo", Repository: "repo-0"},
					{Name: "foo", Repository: "repo-1"},
				},
			},
			ValidationError("more than one dependency with name or alias \"foo\""),
		},
		{
			"dependencies has nil",
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Dependencies: []*Dependency{
					nil,
				},
			},
			ValidationError("dependencies must not contain empty or null nodes"),
		},
		{
			"maintainer not empty",
			&Metadata{
				Name:       "test",
				APIVersion: "v2",
				Version:    "1.0",
				Type:       "application",
				Maintainers: []*Maintainer{
					nil,
				},
			},
			ValidationError("maintainers must not contain empty or null nodes"),
		},
		{
			"version invalid",
			&Metadata{APIVersion: "v2", Name: "test", Version: "1.2.3.4"},
			ValidationError("chart.metadata.version \"1.2.3.4\" is invalid"),
		},
	}

	for _, tt := range tests {
		result := tt.md.Validate()
		if result != tt.err {
			t.Errorf("expected %q, got %q in test %q", tt.err, result, tt.name)
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
