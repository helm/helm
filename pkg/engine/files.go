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

package engine

import (
	"encoding/base64"
	"path"
	"strings"

	"github.com/gobwas/glob"

	"helm.sh/helm/v3/pkg/chart"
)

// files is a map of files in a chart that can be accessed from a template.
type files map[string][]byte

// NewFiles creates a new files from chart files.
// Given an []*chart.File (the format for files in a chart.Chart), extract a map of files.
func newFiles(from []*chart.File) files {
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
func (f files) GetBytes(name string) []byte {
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
func (f files) Get(name string) string {
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
func (f files) Glob(pattern string) files {
	g, err := glob.Compile(pattern, '/')
	if err != nil {
		g, _ = glob.Compile("**")
	}

	nf := newFiles(nil)
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
// (via toYAML function) if it cannot be serialized to YAML, or if the Files
// object is nil.
//
// The output will not be indented, so you will want to pipe this to the
// 'indent' template function.
//
//	data:
//
// {{ .Files.Glob("config/**").AsConfig() | indent 4 }}
func (f files) AsConfig() string {
	if f == nil {
		return ""
	}

	m := make(map[string]string)

	// Explicitly convert to strings, and file names
	for k, v := range f {
		m[path.Base(k)] = string(v)
	}

	return toYAML(m)
}

// AsSecrets returns the base64-encoded value of a Files object suitable for
// including in the 'data' section of a Kubernetes Secret definition.
// Duplicate keys will be overwritten, so be aware that your file names
// (regardless of path) should be unique.
//
// This is designed to be called from a template, and will return empty string
// (via toYAML function) if it cannot be serialized to YAML, or if the Files
// object is nil.
//
// The output will not be indented, so you will want to pipe this to the
// 'indent' template function.
//
//	data:
//
// {{ .Files.Glob("secrets/*").AsSecrets() | indent 4 }}
func (f files) AsSecrets() string {
	if f == nil {
		return ""
	}

	m := make(map[string]string)

	for k, v := range f {
		m[path.Base(k)] = base64.StdEncoding.EncodeToString(v)
	}

	return toYAML(m)
}

// Lines returns each line of a named file (split by "\n") as a slice, so it can
// be ranged over in your templates.
//
// This is designed to be called from a template.
//
// {{ range .Files.Lines "foo/bar.html" }}
// {{ . }}{{ end }}
func (f files) Lines(path string) []string {
	if f == nil || f[path] == nil {
		return []string{}
	}
	s := string(f[path])
	if s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return strings.Split(s, "\n")
}
