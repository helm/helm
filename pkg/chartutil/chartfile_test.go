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

package chartutil

import (
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

const testfile = "testdata/chartfiletest.yaml"

func TestLoadChartfile(t *testing.T) {
	f, err := LoadChartfile(testfile)
	if err != nil {
		t.Errorf("Failed to open %s: %s", testfile, err)
		return
	}
	verifyChartfile(t, f, "frobnitz")
}

func verifyChartfile(t *testing.T, f *chart.Metadata, name string) {

	if f == nil {
		t.Fatal("Failed verifyChartfile because f is nil")
	}

	// Api instead of API because it was generated via protobuf.
	if f.ApiVersion != ApiVersionV1 {
		t.Errorf("Expected API Version %q, got %q", ApiVersionV1, f.ApiVersion)
	}

	if f.Name != name {
		t.Errorf("Expected %s, got %s", name, f.Name)
	}

	if f.Description != "This is a frobnitz." {
		t.Errorf("Unexpected description %q", f.Description)
	}

	if f.Version != "1.2.3" {
		t.Errorf("Unexpected version %q", f.Version)
	}

	if len(f.Maintainers) != 2 {
		t.Errorf("Expected 2 maintainers, got %d", len(f.Maintainers))
	}

	if f.Maintainers[0].Name != "The Helm Team" {
		t.Errorf("Unexpected maintainer name.")
	}

	if f.Maintainers[1].Email != "nobody@example.com" {
		t.Errorf("Unexpected maintainer email.")
	}

	if len(f.Sources) != 1 {
		t.Fatalf("Unexpected number of sources")
	}

	if f.Sources[0] != "https://example.com/foo/bar" {
		t.Errorf("Expected https://example.com/foo/bar, got %s", f.Sources)
	}

	if f.Home != "http://example.com" {
		t.Error("Unexpected home.")
	}

	if f.Icon != "https://example.com/64x64.png" {
		t.Errorf("Unexpected icon: %q", f.Icon)
	}

	if len(f.Keywords) != 3 {
		t.Error("Unexpected keywords")
	}

	if len(f.Annotations) != 2 {
		t.Fatalf("Unexpected annotations")
	}

	if want, got := "extravalue", f.Annotations["extrakey"]; want != got {
		t.Errorf("Want %q, but got %q", want, got)
	}

	if want, got := "anothervalue", f.Annotations["anotherkey"]; want != got {
		t.Errorf("Want %q, but got %q", want, got)
	}

	kk := []string{"frobnitz", "sprocket", "dodad"}
	for i, k := range f.Keywords {
		if kk[i] != k {
			t.Errorf("Expected %q, got %q", kk[i], k)
		}
	}
}

func TestIsChartDir(t *testing.T) {
	validChartDir, err := IsChartDir("testdata/frobnitz")
	if !validChartDir {
		t.Errorf("unexpected error while reading chart-directory: (%v)", err)
		return
	}
	validChartDir, err = IsChartDir("testdata")
	if validChartDir || err == nil {
		t.Errorf("expected error but did not get any")
		return
	}
}
