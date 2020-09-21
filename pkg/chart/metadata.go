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

// Maintainer describes a Chart maintainer.
type Maintainer struct {
	// Name is a user name or organization name
	Name string `json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `json:"email,omitempty"`
	// URL is an optional URL to an address for the named maintainer
	URL string `json:"url,omitempty"`
}

// Metadata for a Chart file. This models the structure of a Chart.yaml file.
type Metadata struct {
	// The name of the chart
	Name string `json:"name,omitempty"`
	// The URL to a relevant project page, git repo, or contact person
	Home string `json:"home,omitempty"`
	// Source is the URL to the source code of this chart
	Sources []string `json:"sources,omitempty"`
	// A SemVer 2 conformant version string of the chart
	Version string `json:"version,omitempty"`
	// A one-sentence description of the chart
	Description string `json:"description,omitempty"`
	// A list of string keywords
	Keywords []string `json:"keywords,omitempty"`
	// A list of name and URL/email address combinations for the maintainer(s)
	Maintainers []*Maintainer `json:"maintainers,omitempty"`
	// The URL to an icon file.
	Icon string `json:"icon,omitempty"`
	// The API Version of this chart.
	APIVersion string `json:"apiVersion,omitempty"`
	// The condition to check to enable chart
	Condition string `json:"condition,omitempty"`
	// The tags to check to enable chart
	Tags string `json:"tags,omitempty"`
	// The version of the application enclosed inside of this chart.
	AppVersion string `json:"appVersion,omitempty"`
	// Whether or not this chart is deprecated
	Deprecated bool `json:"deprecated,omitempty"`
	// Annotations are additional mappings uninterpreted by Helm,
	// made available for inspection by other applications.
	Annotations map[string]string `json:"annotations,omitempty"`
	// KubeVersion is a SemVer constraint specifying the version of Kubernetes required.
	KubeVersion string `json:"kubeVersion,omitempty"`
	// Dependencies are a list of dependencies for a chart.
	Dependencies []*Dependency `json:"dependencies,omitempty"`
	// Specifies the chart type: application or library
	Type string `json:"type,omitempty"`
}

// Validate checks the metadata for known issues, returning an error if metadata is not correct
func (md *Metadata) Validate() error {
	if md == nil {
		return ValidationError("chart.metadata is required")
	}
	if md.APIVersion == "" {
		return ValidationError("chart.metadata.apiVersion is required")
	}
	if md.Name == "" {
		return ValidationError("chart.metadata.name is required")
	}
	if md.Version == "" {
		return ValidationError("chart.metadata.version is required")
	}
	if !isValidChartType(md.Type) {
		return ValidationError("chart.metadata.type must be application or library")
	}

	// Aliases need to be validated here to make sure that the alias name does
	// not contain any illegal characters.
	for _, dependency := range md.Dependencies {
		if err := validateDependency(dependency); err != nil {
			return err
		}
	}

	// TODO validate valid semver here?
	return nil
}

func isValidChartType(in string) bool {
	switch in {
	case "", "application", "library":
		return true
	}
	return false
}

// validateDependency checks for common problems with the dependency datastructure in
// the chart. This check must be done at load time before the dependency's charts are
// loaded.
func validateDependency(dep *Dependency) error {
	if len(dep.Alias) > 0 && !aliasNameFormat.MatchString(dep.Alias) {
		return ValidationErrorf("dependency %q has disallowed characters in the alias", dep.Name)
	}
	return nil
}
