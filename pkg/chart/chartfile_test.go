/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

func TestLoadChartfile(t *testing.T) {
	f, err := LoadChartfile(testfile)
	if err != nil {
		t.Errorf("Failed to open %s: %s", testfile, err)
		return
	}

	if len(f.Environment[0].Extensions) != 2 {
		t.Errorf("Expected two extensions, got %d", len(f.Environment[0].Extensions))
	}

	if f.Name != "frobnitz" {
		t.Errorf("Expected frobnitz, got %s", f.Name)
	}

	if len(f.Maintainers) != 2 {
		t.Errorf("Expected 2 maintainers, got %d", len(f.Maintainers))
	}

	if len(f.Dependencies) != 1 {
		t.Errorf("Expected 2 dependencies, got %d", len(f.Dependencies))
	}

	if f.Dependencies[0].Name != "thingerbob" {
		t.Errorf("Expected second dependency to be thingerbob: %q", f.Dependencies[0].Name)
	}

	if f.Source[0] != "https://example.com/foo/bar" {
		t.Errorf("Expected https://example.com/foo/bar, got %s", f.Source)
	}

	expander := f.Expander
	if expander == nil {
		t.Errorf("No expander found in %s", testfile)
	} else {
		if expander.Name != "Expandybird" {
			t.Errorf("Expected expander name Expandybird, got %s", expander.Name)
		}

		if expander.Entrypoint != "templates/wordpress.jinja" {
			t.Errorf("Expected expander entrypoint templates/wordpress.jinja, got %s", expander.Entrypoint)
		}
	}

	if f.Schema != "wordpress.jinja.schema" {
		t.Errorf("Expected schema wordpress.jinja.schema, got %s", f.Schema)
	}
}

func TestVersionOK(t *testing.T) {
	f, err := LoadChartfile(testfile)
	if err != nil {
		t.Errorf("Error loading %s: %s", testfile, err)
	}

	// These are canaries. The SemVer package exhuastively tests the
	// various  permutations. This will alert us if we wired it up
	// incorrectly.

	d := f.Dependencies[0]
	if d.VersionOK("1.0.0") {
		t.Errorf("1.0.0 should have been marked out of range")
	}

	if !d.VersionOK("3.2.3") {
		t.Errorf("Version 3.2.3 should have been marked in-range")
	}

}
