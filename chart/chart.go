/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package chart

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubernetes/deployment-manager/log"
)

const ChartfileName string = "Chart.yaml"

const (
	preTemplates string = "templates/"
	preHooks     string = "hooks/"
	preDocs      string = "docs/"
	preIcon      string = "icon.svg"
)

// Chart represents a complete chart.
//
// A chart consists of the following parts:
//
// 	- Chart.yaml: In code, we refer to this as the Chartfile
// 	- templates/*: The template directory
// 	- README.md: Optional README file
// 	- LICENSE: Optional license file
// 	- hooks/: Optional hooks registry
//	- docs/: Optional docs directory
//
// Packed charts are stored in gzipped tar archives (.tgz). Unpackaged charts
// are directories where the directory name is the Chartfile.Name.
//
// Optionally, a chart might also locate a provenance (.prov) file that it
// can use for cryptographic signing.
type Chart struct {
	loader chartLoader
}

// Close the chart.
//
// Charts should always be closed when no longer needed.
func (c *Chart) Close() error {
	return c.loader.close()
}

// Chartfile gets the Chartfile (Chart.yaml) for this chart.
func (c *Chart) Chartfile() *Chartfile {
	return c.loader.chartfile()
}

// Dir() returns the directory where the charts are located.
func (c *Chart) Dir() string {
	return c.loader.dir()
}

// DocsDir returns the directory where the chart's documentation is stored.
func (c *Chart) DocsDir() string {
	return filepath.Join(c.loader.dir(), preDocs)
}

// HooksDir returns the directory where the hooks are stored.
func (c *Chart) HooksDir() string {
	return filepath.Join(c.loader.dir(), preHooks)
}

// TemplatesDir returns the directory where the templates are stored.
func (c *Chart) TemplatesDir() string {
	return filepath.Join(c.loader.dir(), preTemplates)
}

// Icon returns the path to the icon.svg file.
//
// If an icon is not found in the chart, this will return an error.
func (c *Chart) Icon() (string, error) {
	i := filepath.Join(c.Dir(), preIcon)
	_, err := os.Stat(i)
	return i, err
}

// chartLoader provides load, close, and save implementations for a chart.
type chartLoader interface {
	// Chartfile resturns a *Chartfile for this chart.
	chartfile() *Chartfile
	// Dir returns a directory where the chart can be accessed.
	dir() string

	// Close cleans up a chart.
	close() error
}

type dirChart struct {
	chartyaml *Chartfile
	chartdir  string
}

func (d *dirChart) chartfile() *Chartfile {
	return d.chartyaml
}

func (d *dirChart) dir() string {
	return d.chartdir
}

func (d *dirChart) close() error {
	return nil
}

type tarChart struct {
	chartyaml *Chartfile
	tmpDir    string
}

func (t *tarChart) chartfile() *Chartfile {
	return t.chartyaml
}

func (t *tarChart) dir() string {
	return t.tmpDir
}

func (t *tarChart) close() error {
	// Remove the temp directory.
	return os.RemoveAll(t.tmpDir)
}

// LoadDir loads an entire chart from a directory.
//
// This includes the Chart.yaml (*Chartfile) and all of the manifests.
//
// If you are just reading the Chart.yaml file, it is substantially more
// performant to use LoadChartfile.
func LoadDir(chart string) (*Chart, error) {
	if fi, err := os.Stat(chart); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("Chart %s is not a directory.", chart)
	}

	cf, err := LoadChartfile(filepath.Join(chart, "Chart.yaml"))
	if err != nil {
		return nil, err
	}

	cl := &dirChart{
		chartyaml: cf,
		chartdir:  chart,
	}

	return &Chart{
		loader: cl,
	}, nil
}

// Load loads a chart from a chart archive.
//
// A chart archive is a gzipped tar archive that follows the Chart format
// specification.
func Load(archive string) (*Chart, error) {
	if fi, err := os.Stat(archive); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("cannot load a directory with chart.Load()")
	}

	raw, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer raw.Close()

	unzipped, err := gzip.NewReader(raw)
	if err != nil {
		return nil, err
	}
	defer unzipped.Close()

	untarred := tar.NewReader(unzipped)
	c, err := loadTar(untarred)
	if err != nil {
		return nil, err
	}

	cf, err := LoadChartfile(filepath.Join(c.tmpDir, ChartfileName))
	if err != nil {
		return nil, err
	}
	c.chartyaml = cf
	return &Chart{loader: c}, nil
}

func loadTar(r *tar.Reader) (*tarChart, error) {
	td, err := ioutil.TempDir("", "chart-")
	if err != nil {
		return nil, err
	}
	c := &tarChart{
		chartyaml: &Chartfile{},
		tmpDir:    td,
	}

	firstDir := ""

	hdr, err := r.Next()
	for err == nil {
		log.Debug("Reading %s", hdr.Name)

		// This is to prevent malformed tar attacks.
		hdr.Name = filepath.Clean(hdr.Name)

		if firstDir == "" {
			fi := hdr.FileInfo()
			if fi.IsDir() {
				log.Debug("Discovered app named %s", hdr.Name)
				firstDir = hdr.Name
			} else {
				log.Warn("Unexpected file at root of archive: %s", hdr.Name)
			}
		} else if strings.HasPrefix(hdr.Name, firstDir) {
			log.Debug("Extracting %s to %s", hdr.Name, c.tmpDir)

			// We know this has the prefix, so we know there won't be an error.
			rel, _ := filepath.Rel(firstDir, hdr.Name)

			// If tar record is a directory, create one in the tmpdir and return.
			if hdr.FileInfo().IsDir() {
				os.MkdirAll(filepath.Join(c.tmpDir, rel), 0755)
				hdr, err = r.Next()
				continue
			}

			dest := filepath.Join(c.tmpDir, rel)
			f, err := os.Create(filepath.Join(c.tmpDir, rel))
			if err != nil {
				log.Warn("Could not create %s: %s", dest, err)
				hdr, err = r.Next()
				continue
			}
			if _, err := io.Copy(f, r); err != nil {
				log.Warn("Failed to copy %s: %s", dest, err)
			}
			f.Close()
		} else {
			log.Warn("Unexpected file outside of chart: %s", hdr.Name)
		}
		hdr, err = r.Next()
	}

	if err != nil && err != io.EOF {
		log.Warn("Unexpected error reading tar: %s", err)
		c.close()
		return c, err
	}
	log.Info("Reached end of Tar file")

	return c, nil
}
