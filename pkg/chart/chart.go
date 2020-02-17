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
	"path/filepath"
	"strings"
)

// APIVersionV1 is the API version number for version 1.
const APIVersionV1 = "v1"

// APIVersionV2 is the API version number for version 2.
const APIVersionV2 = "v2"

// Chart is a helm package that contains metadata, a default config, zero or more
// optionally parameterizable templates, and zero or more charts (dependencies).
type Chart struct {
	// Raw contains the raw contents of the files originally contained in the chart archive.
	//
	// This should not be used except in special cases like `helm show values`,
	// where we want to display the raw values, comments and all.
	Raw []*File `json:"-"`
	// Metadata is the contents of the Chartfile.
	Metadata *Metadata `json:"metadata"`
	// Lock is the contents of Chart.lock.
	Lock *Lock `json:"lock"`
	// Templates for this chart.
	Templates []*File `json:"templates"`
	// Values are default config for this chart.
	Values map[string]interface{} `json:"values"`
	// Schema is an optional JSON schema for imposing structure on Values
	Schema []byte `json:"schema"`
	// Files are miscellaneous files in a chart archive,
	// e.g. README, LICENSE, etc.
	Files []*File `json:"files"`

	parent       *Chart
	dependencies []*Chart
}

type CRD struct {
	// Name is the File.Name for the crd file
	Name string
	// Filename is the File obj Name including (sub-)chart.ChartFullPath
	Filename string
	// File is the File obj for the crd
	File *File
}

// SetDependencies replaces the chart dependencies.
func (ch *Chart) SetDependencies(charts ...*Chart) {
	ch.dependencies = nil
	ch.AddDependency(charts...)
}

// Name returns the name of the chart.
func (ch *Chart) Name() string {
	if ch.Metadata == nil {
		return ""
	}
	return ch.Metadata.Name
}

// AddDependency determines if the chart is a subchart.
func (ch *Chart) AddDependency(charts ...*Chart) {
	for i, x := range charts {
		charts[i].parent = ch
		ch.dependencies = append(ch.dependencies, x)
	}
}

// Root finds the root chart.
func (ch *Chart) Root() *Chart {
	if ch.IsRoot() {
		return ch
	}
	return ch.Parent().Root()
}

// Dependencies are the charts that this chart depends on.
func (ch *Chart) Dependencies() []*Chart { return ch.dependencies }

// IsRoot determines if the chart is the root chart.
func (ch *Chart) IsRoot() bool { return ch.parent == nil }

// Parent returns a subchart's parent chart.
func (ch *Chart) Parent() *Chart { return ch.parent }

// ChartPath returns the full path to this chart in dot notation.
func (ch *Chart) ChartPath() string {
	if !ch.IsRoot() {
		return ch.Parent().ChartPath() + "." + ch.Name()
	}
	return ch.Name()
}

// ChartFullPath returns the full path to this chart.
func (ch *Chart) ChartFullPath() string {
	if !ch.IsRoot() {
		return ch.Parent().ChartFullPath() + "/charts/" + ch.Name()
	}
	return ch.Name()
}

// Validate validates the metadata.
func (ch *Chart) Validate() error {
	return ch.Metadata.Validate()
}

// AppVersion returns the appversion of the chart.
func (ch *Chart) AppVersion() string {
	if ch.Metadata == nil {
		return ""
	}
	return ch.Metadata.AppVersion
}

// CRDs returns a list of File objects in the 'crds/' directory of a Helm chart.
// Deprecated: use CRDObjects()
func (ch *Chart) CRDs() []*File {
	files := []*File{}
	// Find all resources in the crds/ directory
	for _, f := range ch.Files {
		if strings.HasPrefix(f.Name, "crds/") && hasManifestExtension(f.Name) {
			files = append(files, f)
		}
	}
	// Get CRDs from dependencies, too.
	for _, dep := range ch.Dependencies() {
		files = append(files, dep.CRDs()...)
	}
	return files
}

// CRDObjects returns a list of CRD objects in the 'crds/' directory of a Helm chart & subcharts
func (ch *Chart) CRDObjects() []CRD {
	crds := []CRD{}
	// Find all resources in the crds/ directory
	for _, f := range ch.Files {
		if strings.HasPrefix(f.Name, "crds/") && hasManifestExtension(f.Name) {
			mycrd := CRD{Name: f.Name, Filename: filepath.Join(ch.ChartFullPath(), f.Name), File: f}
			crds = append(crds, mycrd)
		}
	}
	// Get CRDs from dependencies, too.
	for _, dep := range ch.Dependencies() {
		crds = append(crds, dep.CRDObjects()...)
	}
	return crds
}

func hasManifestExtension(fname string) bool {
	ext := filepath.Ext(fname)
	return strings.EqualFold(ext, ".yaml") || strings.EqualFold(ext, ".yml") || strings.EqualFold(ext, ".json")
}
