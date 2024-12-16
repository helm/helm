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

import "time"

type PostRendererOptions struct {
	Command string   `json:"command" yaml:"command"`
	Args    []string `json:"args,omitempty" yaml:"args,omitempty"`
}

// Dependency describes a chart upon which another chart depends.
//
// Dependencies can be used to express developer intent, or to capture the state
// of a chart.
type Dependency struct {
	// Name is the name of the dependency.
	//
	// This must mach the name in the dependency's Chart.yaml.
	Name string `json:"name" yaml:"name"`
	// Version is the version (range) of this chart.
	//
	// A lock file will always produce a single version, while a dependency
	// may contain a semantic version range.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// The URL to the repository.
	//
	// Appending `index.yaml` to this string should result in a URL that can be
	// used to fetch the repository index.
	Repository string `json:"repository" yaml:"repository"`
	// A yaml path that resolves to a boolean, used for enabling/disabling charts (e.g. subchart1.enabled )
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
	// Tags can be used to group charts for enabling/disabling together
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	// Enabled bool determines if chart should be loaded
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// ImportValues holds the mapping of source values to parent key to be imported. Each item can be a
	// string or pair of child/parent sublist items.
	ImportValues []interface{} `json:"import-values,omitempty" yaml:"import-values,omitempty"`
	// Alias usable alias to be used for the chart
	Alias string `json:"alias,omitempty" yaml:"alias,omitempty"`
	// A post rendering operation that will be applied to the rendered outputs of this dependency
	PostRenderer *PostRendererOptions `json:"postRenderer,omitempty" yaml:"postRenderer,omitempty"`
}

// Validate checks for common problems with the dependency datastructure in
// the chart. This check must be done at load time before the dependency's charts are
// loaded.
func (d *Dependency) Validate() error {
	if d == nil {
		return ValidationError("dependencies must not contain empty or null nodes")
	}
	d.Name = sanitizeString(d.Name)
	d.Version = sanitizeString(d.Version)
	d.Repository = sanitizeString(d.Repository)
	d.Condition = sanitizeString(d.Condition)
	for i := range d.Tags {
		d.Tags[i] = sanitizeString(d.Tags[i])
	}
	if d.Alias != "" && !aliasNameFormat.MatchString(d.Alias) {
		return ValidationErrorf("dependency %q has disallowed characters in the alias", d.Name)
	}
	return nil
}

// Lock is a lock file for dependencies.
//
// It represents the state that the dependencies should be in.
type Lock struct {
	// Generated is the date the lock file was last generated.
	Generated time.Time `json:"generated"`
	// Digest is a hash of the dependencies in Chart.yaml.
	Digest string `json:"digest"`
	// Dependencies is the list of dependencies that this lock file has locked.
	Dependencies []*Dependency `json:"dependencies"`
}
