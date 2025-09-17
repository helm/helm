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

package loader

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/sympath"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/ignore"
)

var utf8bom = []byte{0xEF, 0xBB, 0xBF}

// DirLoader loads a chart from a directory
type DirLoader struct {
	path string
	opts ChartLoadOptions
}

// NewDirLoader creates a new directory loader with default options
func NewDefaultDirLoader(path string) DirLoader {
	return DirLoader{path: path, opts: DefaultChartLoadOptions}
}

// NewDirLoader creates a new directory loader with custom options
func NewDirLoader(path string, opts ChartLoadOptions) DirLoader {
	return DirLoader{path: path, opts: opts}
}

// Load loads the chart
func (l DirLoader) Load() (*chart.Chart, error) {
	return LoadDir(l.path)
}

// LoadWithOptions loads the chart with custom options
func (l DirLoader) LoadWithOptions() (*chart.Chart, error) {
	return LoadDirWithOptions(l.path, l.opts)
}

// LoadDirWithOptions loads from a directory with default options
func LoadDir(dir string) (*chart.Chart, error) {
	return LoadDirWithOptions(dir, DefaultChartLoadOptions)
}

// LoadDirWithOptions loads from a directory with custom options
func (l DirLoader) LoadDirWithOptions() (*chart.Chart, error) {
	return LoadDirWithOptions(l.path, l.opts)
}

// LoadDir loads from a directory.
//
// This loads charts only from directories.
func LoadDirWithOptions(dir string, opts ChartLoadOptions) (*chart.Chart, error) {
	topdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	// Just used for errors.
	c := &chart.Chart{}

	rules := ignore.Empty()
	ifile := filepath.Join(topdir, ignore.HelmIgnore)
	if _, err := os.Stat(ifile); err == nil {
		r, err := ignore.ParseFile(ifile)
		if err != nil {
			return c, err
		}
		rules = r
	}
	rules.AddDefaults()

	files := []*BufferedFile{}
	topdir += string(filepath.Separator)

	walk := func(name string, fi os.FileInfo, err error) error {
		n := strings.TrimPrefix(name, topdir)
		if n == "" {
			// No need to process top level. Avoid bug with helmignore .* matching
			// empty names. See issue 1779.
			return nil
		}

		// Normalize to / since it will also work on Windows
		n = filepath.ToSlash(n)

		if err != nil {
			return err
		}
		if fi.IsDir() {
			// Directory-based ignore rules should involve skipping the entire
			// contents of that directory.
			if rules.Ignore(n, fi) {
				return filepath.SkipDir
			}
			return nil
		}

		// If a .helmignore file matches, skip this file.
		if rules.Ignore(n, fi) {
			return nil
		}

		// Irregular files include devices, sockets, and other uses of files that
		// are not regular files. In Go they have a file mode type bit set.
		// See https://golang.org/pkg/os/#FileMode for examples.
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("cannot load irregular file %s as it has file mode type bits set", name)
		}

		if fi.Size() > opts.MaxDecompressedFileSize {
			return fmt.Errorf("chart file %q is larger than the maximum file size %d", fi.Name(), opts.MaxDecompressedFileSize)
		}

		data, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", n, err)
		}

		data = bytes.TrimPrefix(data, utf8bom)

		files = append(files, &BufferedFile{Name: n, Data: data})
		return nil
	}
	if err = sympath.Walk(topdir, walk); err != nil {
		return c, err
	}

	return LoadFiles(files)
}
