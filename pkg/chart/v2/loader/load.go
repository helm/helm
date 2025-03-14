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
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

const mergePrefix = "\\*"

// ChartLoader loads a chart.
type ChartLoader interface {
	Load() (*chart.Chart, error)
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
func Load(name string) (*chart.Chart, error) {
	l, err := Loader(name)
	if err != nil {
		return nil, err
	}
	return l.Load()
}

// BufferedFile represents an archive file buffered for later processing.
type BufferedFile struct {
	Name string
	Data []byte
}

// LoadFiles loads from in-memory files.
func LoadFiles(files []*BufferedFile) (*chart.Chart, error) {
	c := new(chart.Chart)
	subcharts := make(map[string][]*BufferedFile)

	// do not rely on assumed ordering of files in the chart and crash
	// if Chart.yaml was not coming early enough to initialize metadata
	for _, f := range files {
		c.Raw = append(c.Raw, &chart.File{Name: f.Name, Data: f.Data})
		if f.Name == "Chart.yaml" {
			if c.Metadata == nil {
				c.Metadata = new(chart.Metadata)
			}
			if err := yaml.Unmarshal(f.Data, c.Metadata); err != nil {
				return c, errors.Wrap(err, "cannot load Chart.yaml")
			}
			// NOTE(bacongobbler): while the chart specification says that APIVersion must be set,
			// Helm 2 accepted charts that did not provide an APIVersion in their chart metadata.
			// Because of that, if APIVersion is unset, we should assume we're loading a v1 chart.
			if c.Metadata.APIVersion == "" {
				c.Metadata.APIVersion = chart.APIVersionV1
			}
		}
	}
	for _, f := range files {
		switch {
		case f.Name == "Chart.yaml":
			// already processed
			continue
		case f.Name == "Chart.lock":
			c.Lock = new(chart.Lock)
			if err := yaml.Unmarshal(f.Data, &c.Lock); err != nil {
				return c, errors.Wrap(err, "cannot load Chart.lock")
			}
		case f.Name == "values.yaml":
			values, err := LoadValues(bytes.NewReader(f.Data))
			if err != nil {
				return c, errors.Wrap(err, "cannot load values.yaml")
			}
			c.Values = values
		case f.Name == "values.schema.json":
			c.Schema = f.Data

		// Deprecated: requirements.yaml is deprecated use Chart.yaml.
		// We will handle it for you because we are nice people
		case f.Name == "requirements.yaml":
			if c.Metadata == nil {
				c.Metadata = new(chart.Metadata)
			}
			if c.Metadata.APIVersion != chart.APIVersionV1 {
				log.Printf("Warning: Dependencies are handled in Chart.yaml since apiVersion \"v2\". We recommend migrating dependencies to Chart.yaml.")
			}
			if err := yaml.Unmarshal(f.Data, c.Metadata); err != nil {
				return c, errors.Wrap(err, "cannot load requirements.yaml")
			}
			if c.Metadata.APIVersion == chart.APIVersionV1 {
				c.Files = append(c.Files, &chart.File{Name: f.Name, Data: f.Data})
			}
		// Deprecated: requirements.lock is deprecated use Chart.lock.
		case f.Name == "requirements.lock":
			c.Lock = new(chart.Lock)
			if err := yaml.Unmarshal(f.Data, &c.Lock); err != nil {
				return c, errors.Wrap(err, "cannot load requirements.lock")
			}
			if c.Metadata == nil {
				c.Metadata = new(chart.Metadata)
			}
			if c.Metadata.APIVersion != chart.APIVersionV1 {
				log.Printf("Warning: Dependency locking is handled in Chart.lock since apiVersion \"v2\". We recommend migrating to Chart.lock.")
			}
			if c.Metadata.APIVersion == chart.APIVersionV1 {
				c.Files = append(c.Files, &chart.File{Name: f.Name, Data: f.Data})
			}

		case strings.HasPrefix(f.Name, "templates/"):
			c.Templates = append(c.Templates, &chart.File{Name: f.Name, Data: f.Data})
		case strings.HasPrefix(f.Name, "charts/"):
			if filepath.Ext(f.Name) == ".prov" {
				c.Files = append(c.Files, &chart.File{Name: f.Name, Data: f.Data})
				continue
			}

			fname := strings.TrimPrefix(f.Name, "charts/")
			cname := strings.SplitN(fname, "/", 2)[0]
			subcharts[cname] = append(subcharts[cname], &BufferedFile{Name: fname, Data: f.Data})
		default:
			c.Files = append(c.Files, &chart.File{Name: f.Name, Data: f.Data})
		}
	}

	if c.Metadata == nil {
		return c, errors.New("Chart.yaml file is missing")
	}

	if err := c.Validate(); err != nil {
		return c, err
	}

	for n, files := range subcharts {
		var sc *chart.Chart
		var err error
		switch {
		case strings.IndexAny(n, "_.") == 0:
			continue
		case filepath.Ext(n) == ".tgz":
			file := files[0]
			if file.Name != n {
				return c, errors.Errorf("error unpacking subchart tar in %s: expected %s, got %s", c.Name(), n, file.Name)
			}
			// Untar the chart and add to c.Dependencies
			sc, err = LoadArchive(bytes.NewBuffer(file.Data))
		default:
			// We have to trim the prefix off of every file, and ignore any file
			// that is in charts/, but isn't actually a chart.
			buff := make([]*BufferedFile, 0, len(files))
			for _, f := range files {
				parts := strings.SplitN(f.Name, "/", 2)
				if len(parts) < 2 {
					continue
				}
				f.Name = parts[1]
				buff = append(buff, f)
			}
			sc, err = LoadFiles(buff)
		}

		if err != nil {
			return c, errors.Wrapf(err, "error unpacking subchart %s in %s", n, c.Name())
		}
		c.AddDependency(sc)
	}

	return c, nil
}

// LoadValues loads values from a reader.
//
// The reader is expected to contain one or more YAML documents, the values of which are merged.
// And the values can be either a chart's default values or a user-supplied values.
func LoadValues(data io.Reader) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	reader := utilyaml.NewYAMLReader(bufio.NewReader(data))
	for {
		currentMap := map[string]interface{}{}
		raw, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrap(err, "error reading yaml document")
		}
		if err := yaml.Unmarshal(raw, &currentMap, func(d *json.Decoder) *json.Decoder {
			d.UseNumber()
			return d
		}); err != nil {
			return nil, errors.Wrap(err, "cannot unmarshal yaml document")
		}

		values = MergeMaps(values, currentMap)
	}
	return values, nil
}

// MergeMaps merges two maps. If a key exists in both maps, the value from b will be used.
// If the value is a map, the maps will be merged recursively.
// If the value is a list, the lists will be merged
func MergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if val, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = MergeMaps(bv, val)
					continue
				}
			}
		} else {
			strippedKey := strings.TrimPrefix(k, mergePrefix)
			if out[k] != nil {
				out[strippedKey] = out[k]
				delete(out, k)
			}
			if sourceList, ok := out[strippedKey].([]any); ok && strings.HasPrefix(k, mergePrefix) {

				_, isMapSlice := sourceList[0].(map[string]any)
				if isMapSlice {
					val, ok := v.([]any)
					if !ok {
						// List is explicitly made null on a subsequent file
						if v == nil {
							delete(out, strippedKey)
							continue
						} else {
							log.Printf("Property \"%s\" mismatch during merge", strippedKey)
							continue
						}
					}

					out[strippedKey] = MergeMapLists(sourceList, val)
					continue
				} else if sourceList, ok := out[strippedKey].([]any); ok {
					if val, ok := v.([]any); ok {
						out[strippedKey] = append(sourceList, val...)
					} else {
						out[strippedKey] = v
					}
					continue
				}
			}

		}
		out[k] = v
	}
	return out
}

// MergeMapLists merges two lists of maps. If a prefix of * is set on a map key,
// that key will be used to de-duplicate/merge with the source map
func MergeMapLists(a, b []any) []any {
	out := a
	for j, mapEntry := range b {
		mapEntry, ok := mapEntry.(map[string]any)
		if !ok {
			continue
		}

		var mergeKey string
		var mergeValue interface{}
		for k, v := range mapEntry {
			if strings.HasPrefix(k, mergePrefix) {
				mergeKey = k
				mergeValue = v
				bj, ok := b[j].(map[string]any)
				if !ok {
					continue
				}
				bj[strings.TrimPrefix(mergeKey, mergePrefix)] = v
				delete(bj, mergeKey)
				break
			}
		}
		if len(mergeKey) > 0 {
			strippedMergeKey := strings.TrimPrefix(mergeKey, mergePrefix)

			for i, sourceMapEntry := range out {
				sourceMapEntry, ok := sourceMapEntry.(map[string]any)
				if !ok {
					continue
				}
				for k, v := range sourceMapEntry {
					if (k == strippedMergeKey || k == mergeKey) && v == mergeValue {
						mergedMapEntry := MergeMaps(sourceMapEntry, mapEntry)
						out[i] = mergedMapEntry
						break
					}
				}
			}
		} else {
			out = append(out, mapEntry)
		}
	}
	return out

}
