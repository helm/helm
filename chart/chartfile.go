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

// Specify environmental constraints.
type EnvConstraint struct {
	Name       string   `yaml:"name"`
	Version    string   `yaml:"version"`
	Extensions []string `yaml:"extensions,omitempty"`
	APIGroups  []string `yaml:"apiGroups,omitempty"`
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
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, b, 0644)
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
