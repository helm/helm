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

package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/internal/chart/v3"
	"helm.sh/helm/v4/pkg/chart/loader/archive"
)

// Expand uncompresses and extracts a chart into the specified directory.
func Expand(dir string, r io.Reader) error {
	files, err := archive.LoadArchiveFiles(r)
	if err != nil {
		return err
	}

	// Get the name of the chart
	var chartName string
	for _, file := range files {
		if file.Name == "Chart.yaml" {
			ch := &chart.Metadata{}
			if err := yaml.Unmarshal(file.Data, ch); err != nil {
				return fmt.Errorf("cannot load Chart.yaml: %w", err)
			}
			chartName = ch.Name
		}
	}
	if chartName == "" {
		return errors.New("chart name not specified")
	}

	// Reject chart names that are POSIX path dot-segments or dot-dot segments or contain path separators.
	// A dot-segment name (e.g. ".") causes the chart path to resolve to the root
	// directory and extraction then to write files directly into that extraction root
	// instead of a per-chart subdirectory.
	if chartName == "." || chartName == ".." {
		return fmt.Errorf("chart name %q is not allowed", chartName)
	}
	if chartName != filepath.Base(chartName) {
		return fmt.Errorf("chart name %q must not contain path separators", chartName)
	}

	// An empty dir means the current directory.
	dir = filepath.Clean(dir)

	// Every write below goes through an os.Root, so files can only be created
	// inside the chart directory, even if dir already contains symlinks or is
	// modified while the chart is being expanded.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.MkdirAll(chartName, 0o755); err != nil {
		return err
	}
	chartRoot, err := root.OpenRoot(chartName)
	if err != nil {
		return err
	}
	defer chartRoot.Close()

	// Defense-in-depth: the chart directory must be a subdirectory of dir,
	// never dir itself.
	rootInfo, err := root.Stat(".")
	if err != nil {
		return err
	}
	chartInfo, err := chartRoot.Stat(".")
	if err != nil {
		return err
	}
	if os.SameFile(rootInfo, chartInfo) {
		return fmt.Errorf("chart name %q resolves to the extraction root", chartName)
	}

	// Copy all files verbatim. We don't parse these files because parsing can remove
	// comments.
	for _, file := range files {
		name := filepath.FromSlash(file.Name)

		// Make sure the necessary subdirs get created.
		if err := chartRoot.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return err
		}

		if err := chartRoot.WriteFile(name, file.Data, 0o644); err != nil {
			return err
		}
	}

	return nil
}

// ExpandFile expands the src file into the dest directory.
func ExpandFile(dest, src string) error {
	h, err := os.Open(src)
	if err != nil {
		return err
	}
	defer h.Close()
	return Expand(dest, h)
}
