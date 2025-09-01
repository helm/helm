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

	"helm.sh/helm/v4/internal/plugin/schema"
)

// Metadata of a plugin, converted from the "on-disk" legacy or v1 plugin.yaml
// Specifically, Config and RuntimeConfig are converted to their respective types based on the plugin type and runtime
type Metadata struct {
	// APIVersion specifies the plugin API version
	APIVersion string

	// Name is the name of the plugin
	Name string

	// Type of plugin (eg, cli/v1, getter/v1, postrenderer/v1)
	Type string

	// Runtime specifies the runtime type (subprocess, wasm)
	Runtime string

	// Version is the SemVer 2 version of the plugin.
	Version string

	// SourceURL is the URL where this plugin can be found
	SourceURL string

	// Config contains the type-specific configuration for this plugin
	Config Config

	// RuntimeConfig contains the runtime-specific configuration
	RuntimeConfig RuntimeConfig
}

func (m Metadata) Validate() error {
	var errs []error

	if !validPluginName.MatchString(m.Name) {
		errs = append(errs, fmt.Errorf("invalid name"))
	}

	if m.APIVersion == "" {
		errs = append(errs, fmt.Errorf("empty APIVersion"))
	}

	if m.Type == "" {
		errs = append(errs, fmt.Errorf("empty type field"))
	}

	if m.Runtime == "" {
		errs = append(errs, fmt.Errorf("empty runtime field"))
	}

	if m.Config == nil {
		errs = append(errs, fmt.Errorf("missing config field"))
	}

	if m.RuntimeConfig == nil {
		errs = append(errs, fmt.Errorf("missing runtimeConfig field"))
	}

	// Validate the config itself
	if m.Config != nil {
		if err := m.Config.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("config validation failed: %w", err))
		}
	}

	// Validate the runtime config itself
	if m.RuntimeConfig != nil {
		if err := m.RuntimeConfig.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("runtime config validation failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func fromMetadataLegacy(m MetadataLegacy) *Metadata {
	pluginType := "cli/v1"

	if len(m.Downloaders) > 0 {
		pluginType = "getter/v1"
	}

	return &Metadata{
		APIVersion:    "legacy",
		Name:          m.Name,
		Version:       m.Version,
		Type:          pluginType,
		Runtime:       "subprocess",
		Config:        buildLegacyConfig(m, pluginType),
		RuntimeConfig: buildLegacyRuntimeConfig(m),
	}
}

func buildLegacyConfig(m MetadataLegacy, pluginType string) Config {
	switch pluginType {
	case "getter/v1":
		var protocols []string
		for _, d := range m.Downloaders {
			protocols = append(protocols, d.Protocols...)
		}
		return &ConfigGetter{
			Protocols: protocols,
		}
	case "cli/v1":
		return &ConfigCLI{
			Usage:       "",            // Legacy plugins don't have Usage field for command syntax
			ShortHelp:   m.Usage,       // Map legacy usage to shortHelp
			LongHelp:    m.Description, // Map legacy description to longHelp
			IgnoreFlags: m.IgnoreFlags,
		}
	default:
		return nil
	}
}

func buildLegacyRuntimeConfig(m MetadataLegacy) RuntimeConfig {
	var protocolCommands []SubprocessProtocolCommand
	if len(m.Downloaders) > 0 {
		protocolCommands =
			make([]SubprocessProtocolCommand, 0, len(m.Downloaders))
		for _, d := range m.Downloaders {
			protocolCommands = append(protocolCommands, SubprocessProtocolCommand(d))
		}
	}
	return &RuntimeConfigSubprocess{
		PlatformCommands: m.PlatformCommands,
		Command:          m.Command,
		PlatformHooks:    m.PlatformHooks,
		Hooks:            m.Hooks,
		ProtocolCommands: protocolCommands,
	}
}

func fromMetadataV1(mv1 MetadataV1) (*Metadata, error) {

	config, err := convertMetadataConfig(mv1.Type, mv1.Config)
	if err != nil {
		return nil, err
	}

	runtimeConfig, err := convertMetdataRuntimeConfig(mv1.Runtime, mv1.RuntimeConfig)
	if err != nil {
		return nil, err
	}

	return &Metadata{
		APIVersion:    mv1.APIVersion,
		Name:          mv1.Name,
		Type:          mv1.Type,
		Runtime:       mv1.Runtime,
		Version:       mv1.Version,
		SourceURL:     mv1.SourceURL,
		Config:        config,
		RuntimeConfig: runtimeConfig,
	}, nil
}

func convertMetadataConfig(pluginType string, configRaw map[string]any) (Config, error) {
	var err error
	var config Config

	switch pluginType {
	case "test/v1":
		config, err = remarshalConfig[*schema.ConfigTestV1](configRaw)
	case "cli/v1":
		config, err = remarshalConfig[*ConfigCLI](configRaw)
	case "getter/v1":
		config, err = remarshalConfig[*ConfigGetter](configRaw)
	case "postrenderer/v1":
		config, err = remarshalConfig[*ConfigPostrenderer](configRaw)
	default:
		return nil, fmt.Errorf("unsupported plugin type: %s", pluginType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config for %s plugin type: %w", pluginType, err)
	}

	return config, nil
}

func convertMetdataRuntimeConfig(runtimeType string, runtimeConfigRaw map[string]any) (RuntimeConfig, error) {
	var runtimeConfig RuntimeConfig
	var err error

	switch runtimeType {
	case "subprocess":
		runtimeConfig, err = remarshalRuntimeConfig[*RuntimeConfigSubprocess](runtimeConfigRaw)
	case "extism/v1":
		runtimeConfig, err = remarshalRuntimeConfig[*RuntimeConfigExtismV1](runtimeConfigRaw)
	default:
		return nil, fmt.Errorf("unsupported plugin runtime type: %q", runtimeType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal runtimeConfig for %s runtime: %w", runtimeType, err)
	}
	return runtimeConfig, nil
}
