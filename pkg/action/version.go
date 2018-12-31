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

package action

import (
	"bytes"
	"fmt"
	"text/template"

	"k8s.io/helm/pkg/version"
)

// Version represents a Helm version action.
type Version struct {
	Formatter VersionFormatter
}

// VersionFormatter takes a version.BuildInfo and returns a printable string.
type VersionFormatter func(info version.BuildInfo) (string, error)

// Run retrieves the Helm version.
func (v *Version) Run() (string, error) {
	bi := version.GetBuildInfo()
	if v.Formatter == nil {
		v.Formatter = DefaultVersion()
	}
	return v.Formatter(bi)
}

// ShortVersion returns a VersionFormatter capable of generating a SemVer representation of the Helm version
func ShortVersion() VersionFormatter {
	return func(v version.BuildInfo) (string, error) {
		commit := v.GitCommit
		if len(commit) >= 7 {
			commit = commit[:7]
		}
		return fmt.Sprintf("%s+g%s", v.Version, commit), nil
	}
}

// TemplateVersion returns a VersionFormatter that uses a template to render the version
func TemplateVersion(tpl string) VersionFormatter {
	return func(v version.BuildInfo) (string, error) {
		renderer, err := template.New("_").Parse(tpl)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		err = renderer.Execute(&buf, v)
		return buf.String(), err
	}
}

// DefaultVersion returns a version formatted like kubectl formats its version.
func DefaultVersion() VersionFormatter {
	return func(v version.BuildInfo) (string, error) {
		return fmt.Sprintf("%#v", v), nil
	}
}
