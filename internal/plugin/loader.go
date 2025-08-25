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

package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// PluginRaw is an "uninitialized" plugin that has not been bound to a runtime
type PluginRaw struct { //nolint:revive
	Metadata Metadata
	Dir      string
}

func peekAPIVersion(r io.Reader) (string, error) {
	type apiVersion struct {
		APIVersion string `yaml:"apiVersion"`
	}

	var v apiVersion
	d := yaml.NewDecoder(r)
	if err := d.Decode(&v); err != nil {
		return "", err
	}

	return v.APIVersion, nil
}

func loadMetadataLegacy(metadataData []byte) (*Metadata, error) {

	var ml MetadataLegacy
	d := yaml.NewDecoder(bytes.NewReader(metadataData))
	if err := d.Decode(&ml); err != nil {
		return nil, err
	}

	if err := ml.Validate(); err != nil {
		return nil, err
	}

	m := fromMetadataLegacy(ml)
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func loadMetadataV1(metadataData []byte) (*Metadata, error) {

	var mv1 MetadataV1
	d := yaml.NewDecoder(bytes.NewReader(metadataData))
	if err := d.Decode(&mv1); err != nil {
		return nil, err
	}

	if err := mv1.Validate(); err != nil {
		return nil, err
	}

	m, err := fromMetadataV1(mv1)
	if err != nil {
		return nil, fmt.Errorf("failed to convert MetadataV1 to Metadata: %w", err)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func loadMetadata(metadataData []byte) (*Metadata, error) {
	apiVersion, err := peekAPIVersion(bytes.NewReader(metadataData))
	if err != nil {
		return nil, fmt.Errorf("failed to peek %s API version: %w", PluginFileName, err)
	}

	switch apiVersion {
	case "": // legacy
		return loadMetadataLegacy(metadataData)
	case "v1":
		return loadMetadataV1(metadataData)
	}

	return nil, fmt.Errorf("invalid plugin apiVersion: %q", apiVersion)
}

func GlobPluginDirs(baseDir string) ([]string, error) {
	// We want baseDir/*/plugin.yaml
	scanpath := filepath.Join(baseDir, "*", PluginFileName)
	matches, err := filepath.Glob(scanpath)
	if err != nil {
		return nil, fmt.Errorf("failed to search for plugins in %q: %w", scanpath, err)
	}

	return matches, nil
}

// LoadDir loads a plugin source from the given directory
func LoadDirRaw(pluginDir string) (*PluginRaw, error) {
	pluginfile := filepath.Join(pluginDir, PluginFileName)
	metadataData, err := os.ReadFile(pluginfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin at %q: %w", pluginfile, err)
	}

	metadata, err := loadMetadata(metadataData)
	if err != nil {
		return nil, err
	}

	pluginRaw := PluginRaw{
		Metadata: *metadata,
		Dir:      pluginDir,
	}

	return &pluginRaw, nil
}

// findFunc is a function that finds plugin directories
type findFunc func(pluginsDir string) ([]string, error)

func NewDirLoader(store *Store, findFn findFunc) *DirLoader {
	return &DirLoader{
		Store:    store,
		FindFunc: findFn,
	}
}

type DirLoader struct {
	Store    *Store
	FindFunc func(string) ([]string, error)
}

func (l *DirLoader) Load(baseDirs []string) error {

	store := NewStore()

	errs := []error{}
	for _, baseDir := range baseDirs {
		slog.Debug("Loading plugins", "directory", baseDir)
		matches, err := l.FindFunc(baseDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to search for plugins in %q: %w", baseDir, err))
			continue
		}

		for _, yamlFile := range matches {
			dir := filepath.Dir(yamlFile)
			plugRaw, err := LoadDirRaw(dir)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to load plugin %q: %w", dir, err))
				continue
			}

			actualPlugRaw, loaded := store.LoadOrStore(plugRaw)
			if loaded {
				errs = append(errs, fmt.Errorf(
					"two plugins claim the name %q at %q and %q",
					plugRaw.Metadata.Name,
					actualPlugRaw.Dir,
					plugRaw.Dir,
				))
				continue
			}
		}
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	// Atomicly replace the store's plugins with the newly loaded plugins
	l.Store.plugins = store.plugins

	return nil
}
