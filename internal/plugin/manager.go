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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Descriptor describes a plugin to find
type Descriptor struct {
	// Name is the name of the plugin
	Name string
	// Type is the type of the plugin (cli/v1, getter/v1, postrenderer/v1, etc)
	Type string
}

// Catalog is an interface for finding plugins
type Catalog interface {
	FindPlugin(Descriptor) (Plugin, error)
	FindPlugins(Descriptor) ([]Plugin, error)
}

// NewStore creates a new, empty plugin store
func NewStore() Store {
	s := Store{}
	s.plugins.Store(&sync.Map{})
	return s
}

// Store is a concurrent access safe store for plugins
// Specifically, it is a wrapper around sync.Map for *PluginRaw
// It uses atomic.Value to allow for safe replacement of the underlying sync.Map
// Providing concurrency safe iteration over all plugins (for filtering), and name-based lookup
type Store struct {
	plugins atomic.Value
}

func (s *Store) Store(pr *PluginRaw) {
	plugins := s.plugins.Load().(*sync.Map)
	plugins.Store(pr.Metadata.Name, pr)
}

func (s *Store) LoadOrStore(pr *PluginRaw) (*PluginRaw, bool) {
	plugins := s.plugins.Load().(*sync.Map)
	actual, loaded := plugins.LoadOrStore(pr.Metadata.Name, pr)
	return actual.(*PluginRaw), loaded
}

func (s *Store) Load(name string) *PluginRaw {
	plugins := s.plugins.Load().(*sync.Map)
	v, ok := plugins.Load(name)
	if !ok {
		return nil
	}
	return v.(*PluginRaw)
}

func (s *Store) Range(cb func(*PluginRaw)) {
	plugins := s.plugins.Load().(*sync.Map)
	plugins.Range(func(_ any, value any) bool {
		cb(value.(*PluginRaw))
		return true
	})
}

func (s *Store) Delete(pluginName string) {
	plugins := s.plugins.Load().(*sync.Map)
	plugins.Delete(pluginName)
}

type Manager struct {
	runtimes map[string]Runtime
	Store    Store
}

// func NewManager(baseDirs []string) *Manager {
func NewManager() *Manager {
	pm := Manager{
		//baseDirs: baseDirs,
		runtimes: map[string]Runtime{},
	}

	return &pm
}

func (m *Manager) RegisterRuntime(runtimeName string, runtime Runtime) {
	m.runtimes[runtimeName] = runtime
}

func (m *Manager) RetriveRuntime(runtimeName string) Runtime {
	return m.runtimes[runtimeName]
}

func (m *Manager) Catalog() Catalog {
	return &PluginManagerCatalog{Manager: m}
}

func (m *Manager) FindPluginsRaw(filterFn filterFunc) []*PluginRaw {
	results := make([]*PluginRaw, 0, 10)
	m.Store.Range(func(pluginRaw *PluginRaw) {
		if filterFn(&pluginRaw.Metadata) {
			results = append(results, pluginRaw)
		}
	})

	return results
}

func (m *Manager) CreatePlugin(pluginRaw *PluginRaw) (Plugin, error) {
	rt, ok := m.runtimes[pluginRaw.Metadata.Runtime]
	if !ok {
		return nil, fmt.Errorf("unsupported plugin runtime type: %q", pluginRaw.Metadata.Runtime)
	}

	return rt.CreatePlugin(pluginRaw.Dir, &pluginRaw.Metadata)
}

// filterFunc is a function that filters plugins
type filterFunc func(m *Metadata) bool

// makeDescriptorFilter creates a filter function from a descriptor
// Additional plugin filter criteria we wish to support can be added here
func makeDescriptorFilter(descriptor Descriptor) filterFunc {
	return func(m *Metadata) bool {
		// If name is specified, it must match
		if descriptor.Name != "" && m.Name != descriptor.Name {
			return false

		}
		// If type is specified, it must match
		if descriptor.Type != "" && m.Type != descriptor.Type {
			return false
		}

		return true
	}
}

type PluginManagerCatalog struct {
	Manager *Manager
}

func (c *PluginManagerCatalog) FindPlugin(d Descriptor) (Plugin, error) {
	filterFn := makeDescriptorFilter(d)

	pluginsRaw := c.Manager.FindPluginsRaw(filterFn)

	if len(pluginsRaw) == 0 {
		return nil, nil
	}
	if len(pluginsRaw) > 1 {
		return nil, fmt.Errorf("multiple matching plugins found")
	}

	return c.Manager.CreatePlugin(pluginsRaw[0])
}

func (c *PluginManagerCatalog) FindPlugins(d Descriptor) ([]Plugin, error) {
	filterFn := makeDescriptorFilter(d)

	pluginsRaw := make([]*PluginRaw, 0, 10)
	c.Manager.Store.Range(func(pluginRaw *PluginRaw) {
		if filterFn(&pluginRaw.Metadata) {
			pluginsRaw = append(pluginsRaw, pluginRaw)
		}
	})

	results := make([]Plugin, 0, len(pluginsRaw))
	errs := []error{}
	for _, pr := range pluginsRaw {
		p, err := c.Manager.CreatePlugin(pr)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		results = append(results, p)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return results, nil
}

// NewEmptyCatalog returns a Catalog that has no plugins
func NewEmptyCatalog() Catalog {
	return &emptyCatalog{}
}

type emptyCatalog struct{}

func (*emptyCatalog) FindPlugin(Descriptor) (Plugin, error) {
	return nil, nil
}

func (*emptyCatalog) FindPlugins(Descriptor) ([]Plugin, error) {
	return []Plugin{}, nil
}
