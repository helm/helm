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
	"io/ioutil"

	"github.com/Masterminds/semver"
	"gopkg.in/yaml.v2"
)

// Chartfile describes a Helm Chart (e.g. Chart.yaml)
type Chartfile struct {
	Name         string           `yaml:"name"`
	Description  string           `yaml:"description"`
	Version      string           `yaml:"version"`
	Keywords     []string         `yaml:"keywords,omitempty"`
	Maintainers  []*Maintainer    `yaml:"maintainers,omitempty"`
	Source       []string         `yaml:"source,omitempty"`
	Home         string           `yaml:"home"`
	Dependencies []*Dependency    `yaml:"dependencies,omitempty"`
	Environment  []*EnvConstraint `yaml:"environment,omitempty"`
	Expander     *Expander        `yaml:"expander,omitempty"`
	Schema       string           `yaml:"schema,omitempty"`
}

// Maintainer describes a chart maintainer.
type Maintainer struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
}

// Dependency describes a specific dependency.
type Dependency struct {
	Name     string `yaml:"name,omitempty"`
	Version  string `yaml:"version"`
	Location string `yaml:"location"`
}

// EnvConstraint specifies environmental constraints.
type EnvConstraint struct {
	Name       string   `yaml:"name"`
	Version    string   `yaml:"version"`
	Extensions []string `yaml:"extensions,omitempty"`
	APIGroups  []string `yaml:"apiGroups,omitempty"`
}

// Expander controls how template/ is evaluated.
type Expander struct {
	// Kubernetes service name to look up in DNS.
	Name string `json:"name"`
	// During evaluation, which file to start from.
	Entrypoint string `json:"entrypoint"`
}

// LoadChartfile loads a Chart.yaml file into a *Chart.
func LoadChartfile(filename string) (*Chartfile, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var y Chartfile
	return &y, yaml.Unmarshal(b, &y)
}

// Save saves a Chart.yaml file
func (c *Chartfile) Save(filename string) error {
	b, err := c.Marshal()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, b, 0644)
}

// Marshal encodes the chart file into YAML.
func (c *Chartfile) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

// VersionOK returns true if the given version meets the constraints.
//
// It returns false if the version string or constraint is unparsable or if the
// version does not meet the constraint.
func (d *Dependency) VersionOK(version string) bool {
	c, err := semver.NewConstraint(d.Version)
	if err != nil {
		return false
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	return c.Check(v)
}
