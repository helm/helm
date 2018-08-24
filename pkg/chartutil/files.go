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

package chartutil

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/ghodss/yaml"
	"github.com/gobwas/glob"

	"k8s.io/helm/pkg/chart"
)

// Files is a map of files in a chart that can be accessed from a template.
type Files map[string][]byte

// NewFiles creates a new Files from chart files.
// Given an []*any.Any (the format for files in a chart.Chart), extract a map of files.
func NewFiles(from []*chart.File) Files {
	files := make(map[string][]byte)
	for _, f := range from {
		files[f.Name] = f.Data
	}
	return files
}

// GetBytes gets a file by path.
//
// The returned data is raw. In a template context, this is identical to calling
// {{index .Files $path}}.
//
// This is intended to be accessed from within a template, so a missed key returns
// an empty []byte.
func (f Files) GetBytes(name string) []byte {
	if v, ok := f[name]; ok {
		return v
	}
	return []byte{}
}

// Get returns a string representation of the given file.
//
// Fetch the contents of a file as a string. It is designed to be called in a
// template.
//
//	{{.Files.Get "foo"}}
func (f Files) Get(name string) string {
	return string(f.GetBytes(name))
}

// Glob takes a glob pattern and returns another files object only containing
// matched  files.
//
// This is designed to be called from a template.
//
// {{ range $name, $content := .Files.Glob("foo/**") }}
// {{ $name }}: |
// {{ .Files.Get($name) | indent 4 }}{{ end }}
func (f Files) Glob(pattern string) Files {
	g, err := glob.Compile(pattern, '/')
	if err != nil {
		g, _ = glob.Compile("**")
	}

	nf := NewFiles(nil)
	for name, contents := range f {
		if g.Match(name) {
			nf[name] = contents
		}
	}

	return nf
}

// AsConfig turns a Files group and flattens it to a YAML map suitable for
// including in the 'data' section of a Kubernetes ConfigMap definition.
// Duplicate keys will be overwritten, so be aware that your file names
// (regardless of path) should be unique.
//
// This is designed to be called from a template, and will return empty string
// (via ToYAML function) if it cannot be serialized to YAML, or if the Files
// object is nil.
//
// The output will not be indented, so you will want to pipe this to the
// 'indent' template function.
//
//   data:
// {{ .Files.Glob("config/**").AsConfig() | indent 4 }}
func (f Files) AsConfig() string {
	if f == nil {
		return ""
	}

	m := make(map[string]string)

	// Explicitly convert to strings, and file names
	for k, v := range f {
		m[path.Base(k)] = string(v)
	}

	return ToYAML(m)
}

// AsSecrets returns the base64-encoded value of a Files object suitable for
// including in the 'data' section of a Kubernetes Secret definition.
// Duplicate keys will be overwritten, so be aware that your file names
// (regardless of path) should be unique.
//
// This is designed to be called from a template, and will return empty string
// (via ToYAML function) if it cannot be serialized to YAML, or if the Files
// object is nil.
//
// The output will not be indented, so you will want to pipe this to the
// 'indent' template function.
//
//   data:
// {{ .Files.Glob("secrets/*").AsSecrets() }}
func (f Files) AsSecrets() string {
	if f == nil {
		return ""
	}

	m := make(map[string]string)

	for k, v := range f {
		m[path.Base(k)] = base64.StdEncoding.EncodeToString(v)
	}

	return ToYAML(m)
}

// Lines returns each line of a named file (split by "\n") as a slice, so it can
// be ranged over in your templates.
//
// This is designed to be called from a template.
//
// {{ range .Files.Lines "foo/bar.html" }}
// {{ . }}{{ end }}
func (f Files) Lines(path string) []string {
	if f == nil || f[path] == nil {
		return []string{}
	}

	return strings.Split(string(f[path]), "\n")
}

// ToYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func ToYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

// FromYAML converts a YAML document into a map[string]interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func FromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// ToTOML takes an interface, marshals it to toml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func ToTOML(v interface{}) string {
	b := bytes.NewBuffer(nil)
	e := toml.NewEncoder(b)
	err := e.Encode(v)
	if err != nil {
		return err.Error()
	}
	return b.String()
}

// ToJSON takes an interface, marshals it to json, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func ToJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return string(data)
}

// FromJSON converts a JSON document into a map[string]interface{}.
//
// This is not a general-purpose JSON parser, and will not parse all valid
// JSON documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func FromJSON(str string) map[string]interface{} {
	m := make(map[string]interface{})

	if err := json.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}
