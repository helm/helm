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

// FileLoader loads a chart from a file
type FileLoader string

// Load loads a chart
func (l FileLoader) Load() (*chart.Chart, error) {
	return LoadFile(string(l))
}

// LoadFile loads from an archive file.
func LoadFile(name string) (*chart.Chart, error) {
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

	c, err := LoadArchive(raw)
	if err != nil {
		if errors.Is(err, gzip.ErrHeader) {
			return nil, fmt.Errorf("file '%s' does not appear to be a valid chart file (details: %s)", name, err)
		}
	}
	return c, err
}

// LoadArchive loads from a reader containing a compressed tar archive.
func LoadArchive(in io.Reader) (*chart.Chart, error) {
	files, err := archive.LoadArchiveFiles(in)
	if err != nil {
		return nil, err
	}

	return LoadFiles(files)
}
