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
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/pkg/chart/loader/archive"
)

// FileLoader loads a chart from a file with embedded options
type FileLoader struct {
	path string
	opts archive.Options
}

// Load loads a chart
func (l FileLoader) Load() (*chart.Chart, error) {
	return LoadFile(l.path)
}

// NewFileLoader creates a file loader with custom options
func NewFileLoader(path string, opts archive.Options) FileLoader {
	return FileLoader{path: path, opts: opts}
}

// NewDefaultFileLoader creates a file loader with default options
func NewDefaultFileLoader(path string) FileLoader {
	return FileLoader{path: path, opts: archive.DefaultOptions}
}

// LoadFile loads from an archive file with default options
func LoadFile(name string) (*chart.Chart, error) {
	return LoadFileWithOptions(name, archive.DefaultOptions)
}

// LoadFile loads from an archive file with the provided options
func LoadFileWithOptions(name string, opts archive.Options) (*chart.Chart, error) {
	if fi, err := os.Stat(name); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("cannot load a directory")
	}

	raw, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer raw.Close()

	err = archive.EnsureArchive(name, raw)
	if err != nil {
		return nil, err
	}

	c, err := LoadArchiveWithOptions(raw, opts)
	if err != nil {
		if errors.Is(err, gzip.ErrHeader) {
			return nil, fmt.Errorf("file '%s' does not appear to be a valid chart file (details: %w)", name, err)
		}
	}
	return c, err
}

// LoadArchive loads from a reader containing a compressed tar archive.
func LoadArchive(in io.Reader) (*chart.Chart, error) {
	return LoadArchiveWithOptions(in, archive.DefaultOptions)
}

// LoadArchive loads from a reader containing a compressed tar archive with the provided options.
func LoadArchiveWithOptions(in io.Reader, opts archive.Options) (*chart.Chart, error) {
	files, err := archive.LoadArchiveFilesWithOptions(in, opts)
	if err != nil {
		return nil, err
	}

	return LoadFiles(files)
}
