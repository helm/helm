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
	"path/filepath"

	"sigs.k8s.io/yaml"

	c3 "helm.sh/helm/v4/internal/chart/v3"
	c3load "helm.sh/helm/v4/internal/chart/v3/loader"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader/archive"
	c2 "helm.sh/helm/v4/pkg/chart/v2"
	c2load "helm.sh/helm/v4/pkg/chart/v2/loader"
)

// ChartLoader loads a chart.
type ChartLoader interface {
	Load() (chart.Charter, error)
}

// Loader returns a new ChartLoader appropriate for the given chart name
func Loader(name string) (ChartLoader, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return DirLoader(name), nil
	}
	return FileLoader(name), nil
}

// Load takes a string name, tries to resolve it to a file or directory, and then loads it.
//
// This is the preferred way to load a chart. It will discover the chart encoding
// and hand off to the appropriate chart reader.
//
// If a .helmignore file is present, the directory loader will skip loading any files
// matching it. But .helmignore is not evaluated when reading out of an archive.
func Load(name string) (chart.Charter, error) {
	l, err := Loader(name)
	if err != nil {
		return nil, err
	}
	return l.Load()
}

// DirLoader loads a chart from a directory
type DirLoader string

// Load loads the chart
func (l DirLoader) Load() (chart.Charter, error) {
	return LoadDir(string(l))
}

func LoadDir(dir string) (chart.Charter, error) {
	topdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	name := filepath.Join(topdir, "Chart.yaml")
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to detect chart at %s: %w", name, err)
	}

	c := new(chartBase)
	err = yaml.Unmarshal(data, c)
	if err != nil {
		return nil, fmt.Errorf("cannot load Chart.yaml: %w", err)
	}

	switch c.APIVersion {
	case c2.APIVersionV1, c2.APIVersionV2, "":
		return c2load.Load(dir)
	case c3.APIVersionV3:
		return c3load.Load(dir)
	default:
		return nil, errors.New("unsupported chart version")
	}

}

// FileLoader loads a chart from a file
type FileLoader string

// Load loads a chart
func (l FileLoader) Load() (chart.Charter, error) {
	return LoadFile(string(l))
}

func LoadFile(name string) (chart.Charter, error) {
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

	files, err := archive.LoadArchiveFiles(raw)
	if err != nil {
		if errors.Is(err, gzip.ErrHeader) {
			return nil, fmt.Errorf("file '%s' does not appear to be a valid chart file (details: %w)", name, err)
		}
		return nil, errors.New("unable to load chart archive")
	}

	for _, f := range files {
		if f.Name == "Chart.yaml" {
			c := new(chartBase)
			if err := yaml.Unmarshal(f.Data, c); err != nil {
				return c, fmt.Errorf("cannot load Chart.yaml: %w", err)
			}
			switch c.APIVersion {
			case c2.APIVersionV1, c2.APIVersionV2, "":
				return c2load.Load(name)
			case c3.APIVersionV3:
				return c3load.Load(name)
			default:
				return nil, errors.New("unsupported chart version")
			}
		}
	}

	return nil, errors.New("unable to detect chart version, no Chart.yaml found")
}

// LoadArchive loads from a reader containing a compressed tar archive.
func LoadArchive(in io.Reader) (chart.Charter, error) {
	// Note: This function is for use by SDK users such as Flux.

	files, err := archive.LoadArchiveFiles(in)
	if err != nil {
		if errors.Is(err, gzip.ErrHeader) {
			return nil, fmt.Errorf("stream does not appear to be a valid chart file (details: %w)", err)
		}
		return nil, fmt.Errorf("unable to load chart archive: %w", err)
	}

	for _, f := range files {
		if f.Name == "Chart.yaml" {
			c := new(chartBase)
			if err := yaml.Unmarshal(f.Data, c); err != nil {
				return c, fmt.Errorf("cannot load Chart.yaml: %w", err)
			}
			switch c.APIVersion {
			case c2.APIVersionV1, c2.APIVersionV2, "":
				return c2load.LoadFiles(files)
			case c3.APIVersionV3:
				return c3load.LoadFiles(files)
			default:
				return nil, errors.New("unsupported chart version")
			}
		}
	}

	return nil, errors.New("unable to detect chart version, no Chart.yaml found")
}

// chartBase is used to detect the API Version for the chart to run it through the
// loader for that type.
type chartBase struct {
	APIVersion string `json:"apiVersion,omitempty"`
}
