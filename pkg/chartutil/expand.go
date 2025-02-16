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
	"io"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
)

// Expand uncompresses and extracts a chart into the specified directory.
func Expand(dir string, r io.Reader) error {
	files, err := loader.LoadArchiveFiles(r)
	if err != nil {
		return err
	}

	// Get the name of the chart
	var chartName string
	for _, file := range files {
		if file.Name == "Chart.yaml" {
			ch := &chart.Metadata{}
			if err := yaml.Unmarshal(file.Data, ch); err != nil {
				return errors.Wrap(err, "cannot load Chart.yaml")
			}
			chartName = ch.Name
		}
	}
	if chartName == "" {
		return errors.New("chart name not specified")
	}

	// Find the base directory
	// The directory needs to be cleaned prior to passing to SecureJoin or the location may end up
	// being wrong or returning an error. This was introduced in v0.4.0.
	dir = filepath.Clean(dir)
	chartdir, err := securejoin.SecureJoin(dir, chartName)
	if err != nil {
		return err
	}

	// Copy all files verbatim. We don't parse these files because parsing can remove
	// comments.
	for _, file := range files {
		outpath, err := securejoin.SecureJoin(chartdir, file.Name)
		if err != nil {
			return err
		}

		// Make sure the necessary subdirs get created.
		basedir := filepath.Dir(outpath)
		if err := os.MkdirAll(basedir, 0755); err != nil {
			return err
		}

		if err := os.WriteFile(outpath, file.Data, 0644); err != nil {
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
