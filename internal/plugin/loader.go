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
	"fmt"
	"io"
	"os"
	"path/filepath"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero"
	"go.yaml.in/yaml/v3"

	"helm.sh/helm/v4/pkg/helmpath"
)

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
	// NOTE: No strict unmarshalling for legacy plugins - maintain backwards compatibility
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
	d.KnownFields(true)
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

type prototypePluginManager struct {
	runtimes map[string]Runtime
}

func newPrototypePluginManager() (*prototypePluginManager, error) {

	cc, err := wazero.NewCompilationCacheWithDir(helmpath.CachePath("wazero-build"))
	if err != nil {
		return nil, fmt.Errorf("failed to create wazero compilation cache: %w", err)
	}

	return &prototypePluginManager{
		runtimes: map[string]Runtime{
			"subprocess": &RuntimeSubprocess{},
			"extism/v1": &RuntimeExtismV1{
				HostFunctions:    map[string]extism.HostFunction{},
				CompilationCache: cc,
			},
		},
	}, nil
}

func (pm *prototypePluginManager) RegisterRuntime(runtimeName string, runtime Runtime) {
	pm.runtimes[runtimeName] = runtime
}

func (pm *prototypePluginManager) CreatePlugin(pluginPath string, metadata *Metadata) (Plugin, error) {
	rt, ok := pm.runtimes[metadata.Runtime]
	if !ok {
		return nil, fmt.Errorf("unsupported plugin runtime type: %q", metadata.Runtime)
	}

	return rt.CreatePlugin(pluginPath, metadata)
}

// LoadDir loads a plugin from the given directory.
func LoadDir(dirname string) (Plugin, error) {
	pluginfile := filepath.Join(dirname, PluginFileName)
	metadataData, err := os.ReadFile(pluginfile)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin at %q: %w", pluginfile, err)
	}

	m, err := loadMetadata(metadataData)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin %q: %w", dirname, err)
	}

	pm, err := newPrototypePluginManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin manager: %w", err)
	}
	return pm.CreatePlugin(dirname, m)
}

// LoadAll loads all plugins found beneath the base directory.
//
// This scans only one directory level.
func LoadAll(basedir string) ([]Plugin, error) {
	var plugins []Plugin
	// We want basedir/*/plugin.yaml
	scanpath := filepath.Join(basedir, "*", PluginFileName)
	matches, err := filepath.Glob(scanpath)
	if err != nil {
		return nil, fmt.Errorf("failed to search for plugins in %q: %w", scanpath, err)
	}

	// empty dir should load
	if len(matches) == 0 {
		return plugins, nil
	}

	for _, yamlFile := range matches {
		dir := filepath.Dir(yamlFile)
		p, err := LoadDir(dir)
		if err != nil {
			return plugins, err
		}
		plugins = append(plugins, p)
	}
	return plugins, detectDuplicates(plugins)
}

// findFunc is a function that finds plugins in a directory
type findFunc func(pluginsDir string) ([]Plugin, error)

// filterFunc is a function that filters plugins
type filterFunc func(Plugin) bool

// FindPlugins returns a list of plugins that match the descriptor
func FindPlugins(pluginsDirs []string, descriptor Descriptor) ([]Plugin, error) {
	return findPlugins(pluginsDirs, LoadAll, makeDescriptorFilter(descriptor))
}

// findPlugins is the internal implementation that uses the find and filter functions
func findPlugins(pluginsDirs []string, findFn findFunc, filterFn filterFunc) ([]Plugin, error) {
	var found []Plugin
	for _, pluginsDir := range pluginsDirs {
		ps, err := findFn(pluginsDir)

		if err != nil {
			return nil, err
		}

		for _, p := range ps {
			if filterFn(p) {
				found = append(found, p)
			}
		}

	}

	return found, nil
}

// makeDescriptorFilter creates a filter function from a descriptor
// Additional plugin filter criteria we wish to support can be added here
func makeDescriptorFilter(descriptor Descriptor) filterFunc {
	return func(p Plugin) bool {
		// If name is specified, it must match
		if descriptor.Name != "" && p.Metadata().Name != descriptor.Name {
			return false

		}
		// If type is specified, it must match
		if descriptor.Type != "" && p.Metadata().Type != descriptor.Type {
			return false
		}
		return true
	}
}

// FindPlugin returns a single plugin that matches the descriptor
func FindPlugin(dirs []string, descriptor Descriptor) (Plugin, error) {
	plugins, err := FindPlugins(dirs, descriptor)
	if err != nil {
		return nil, err
	}

	if len(plugins) > 0 {
		return plugins[0], nil
	}

	return nil, fmt.Errorf("plugin: %+v not found", descriptor)
}

func detectDuplicates(plugs []Plugin) error {
	names := map[string]string{}

	for _, plug := range plugs {
		if oldpath, ok := names[plug.Metadata().Name]; ok {
			return fmt.Errorf(
				"two plugins claim the name %q at %q and %q",
				plug.Metadata().Name,
				oldpath,
				plug.Dir(),
			)
		}
		names[plug.Metadata().Name] = plug.Dir()
	}

	return nil
}
