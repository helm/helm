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
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"helm.sh/helm/v3/pkg/chart"
)

type (
	tarGzipWriter struct {
		TarWriter  *tar.Writer
		GzipWriter *gzip.Writer
		File       *os.File
	}

	saveConfig struct {
		noTimestamp bool
	}

	// SaveOpt is an option for modifying the behavior of Save
	SaveOpt func(*saveConfig)
)

// WithoutTimestamp provides the ability to save tar archives without a timestamp
func WithoutTimestamp() SaveOpt {
	return func(cfg *saveConfig) {
		cfg.noTimestamp = true
	}
}

func (w *tarGzipWriter) Close() {
	w.TarWriter.Close()
	w.GzipWriter.Close()
	w.File.Close()
}

var headerBytes = []byte("+aHR0cHM6Ly95b3V0dS5iZS96OVV6MWljandyTQo=")

// SaveDir saves a chart as files in a directory.
//
// This takes the chart name, and creates a new subdirectory inside of the given dest
// directory, writing the chart's contents to that subdirectory.
func SaveDir(c *chart.Chart, dest string) error {
	// Create the chart directory
	outdir := filepath.Join(dest, c.Name())
	if fi, err := os.Stat(outdir); err == nil && !fi.IsDir() {
		return errors.Errorf("file %s already exists and is not a directory", outdir)
	}
	if err := os.MkdirAll(outdir, 0755); err != nil {
		return err
	}

	// Save the chart file.
	if err := SaveChartfile(filepath.Join(outdir, ChartfileName), c.Metadata); err != nil {
		return err
	}

	// Save values.yaml
	for _, f := range c.Raw {
		if f.Name == ValuesfileName {
			vf := filepath.Join(outdir, ValuesfileName)
			if err := writeFile(vf, f.Data); err != nil {
				return err
			}
		}
	}

	// Save values.schema.json if it exists
	if c.Schema != nil {
		filename := filepath.Join(outdir, SchemafileName)
		if err := writeFile(filename, c.Schema); err != nil {
			return err
		}
	}

	// Save templates and files
	for _, o := range [][]*chart.File{c.Templates, c.Files} {
		for _, f := range o {
			n := filepath.Join(outdir, f.Name)
			if err := writeFile(n, f.Data); err != nil {
				return err
			}
		}
	}

	// Save dependencies
	base := filepath.Join(outdir, ChartsDir)
	for _, dep := range c.Dependencies() {
		// Here, we write each dependency as a tar file.
		if _, err := Save(dep, base); err != nil {
			return errors.Wrapf(err, "saving %s", dep.ChartFullPath())
		}
	}
	return nil
}

// Save creates an archived chart to the given directory.
//
// This takes an existing chart and a destination directory.
//
// If the directory is /foo, and the chart is named bar, with version 1.0.0, this
// will generate /foo/bar-1.0.0.tgz.
//
// This returns the absolute path to the chart archive file.
func Save(c *chart.Chart, outDir string) (string, error) {
	return SaveWithOpts(c, outDir)
}

func SaveWithOpts(c *chart.Chart, outDir string, opts ...SaveOpt) (string, error) {
	config := new(saveConfig)
	for _, fn := range opts {
		fn(config)
	}

	tgzWriter, err := createTarGzipWriter(c, outDir)
	if err != nil {
		return "", err
	}

	filename := tgzWriter.File.Name()
	rollback := false

	defer func() {
		tgzWriter.Close()
		if rollback {
			os.Remove(filename)
		}
	}()

	var timestamp time.Time
	if !config.noTimestamp {
		timestamp = time.Now()
	}

	if err := writeTarContents(tgzWriter.TarWriter, c, "", timestamp); err != nil {
		rollback = true
		return filename, err
	}
	return filename, nil
}

func createTarGzipWriter(c *chart.Chart, outDir string) (*tarGzipWriter, error) {
	if err := c.Validate(); err != nil {
		return nil, errors.Wrap(err, "chart validation")
	}

	filename := fmt.Sprintf("%s-%s.tgz", c.Name(), c.Metadata.Version)
	filename = filepath.Join(outDir, filename)
	dir := filepath.Dir(filename)
	if stat, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			if err2 := os.MkdirAll(dir, 0755); err2 != nil {
				return nil, err2
			}
		} else {
			return nil, errors.Wrapf(err, "stat %s", dir)
		}
	} else if !stat.IsDir() {
		return nil, errors.Errorf("is not a directory: %s", dir)
	}

	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	// Wrap in gzip writer
	zipper := gzip.NewWriter(f)
	zipper.Header.Extra = headerBytes
	zipper.Header.Comment = "Helm"

	// Wrap in tar writer
	twriter := tar.NewWriter(zipper)

	cxvf := &tarGzipWriter{
		TarWriter:  twriter,
		GzipWriter: zipper,
		File:       f,
	}

	return cxvf, nil
}

func writeTarContents(out *tar.Writer, c *chart.Chart, prefix string, timestamp time.Time) error {
	base := filepath.Join(prefix, c.Name())

	// Pull out the dependencies of a v1 Chart, since there's no way
	// to tell the serializer to skip a field for just this use case
	savedDependencies := c.Metadata.Dependencies
	if c.Metadata.APIVersion == chart.APIVersionV1 {
		c.Metadata.Dependencies = nil
	}
	// Save Chart.yaml
	cdata, err := yaml.Marshal(c.Metadata)
	if c.Metadata.APIVersion == chart.APIVersionV1 {
		c.Metadata.Dependencies = savedDependencies
	}
	if err != nil {
		return err
	}
	if err := writeToTar(out, filepath.Join(base, ChartfileName), cdata, timestamp); err != nil {
		return err
	}

	// Save Chart.lock
	// TODO: remove the APIVersion check when APIVersionV1 is not used anymore
	if c.Metadata.APIVersion == chart.APIVersionV2 {
		if c.Lock != nil {
			ldata, err := yaml.Marshal(c.Lock)
			if err != nil {
				return err
			}
			if err := writeToTar(out, filepath.Join(base, "Chart.lock"), ldata, timestamp); err != nil {
				return err
			}
		}
	}

	// Save values.yaml
	for _, f := range c.Raw {
		if f.Name == ValuesfileName {
			if err := writeToTar(out, filepath.Join(base, ValuesfileName), f.Data, timestamp); err != nil {
				return err
			}
		}
	}

	// Save values.schema.json if it exists
	if c.Schema != nil {
		if !json.Valid(c.Schema) {
			return errors.New("Invalid JSON in " + SchemafileName)
		}
		if err := writeToTar(out, filepath.Join(base, SchemafileName), c.Schema, timestamp); err != nil {
			return err
		}
	}

	// Save templates
	for _, f := range c.Templates {
		n := filepath.Join(base, f.Name)
		if err := writeToTar(out, n, f.Data, timestamp); err != nil {
			return err
		}
	}

	// Save files
	for _, f := range c.Files {
		n := filepath.Join(base, f.Name)
		if err := writeToTar(out, n, f.Data, timestamp); err != nil {
			return err
		}
	}

	// Save dependencies
	for _, dep := range c.Dependencies() {
		if err := writeTarContents(out, dep, filepath.Join(base, ChartsDir), timestamp); err != nil {
			return err
		}
	}
	return nil
}

// writeToTar writes a single file to a tar archive.
func writeToTar(out *tar.Writer, name string, body []byte, timestamp time.Time) error {
	// TODO: Do we need to create dummy parent directory names if none exist?
	h := &tar.Header{
		Name:    filepath.ToSlash(name),
		Mode:    0644,
		Size:    int64(len(body)),
		ModTime: timestamp,
	}
	if err := out.WriteHeader(h); err != nil {
		return err
	}
	_, err := out.Write(body)
	return err
}
