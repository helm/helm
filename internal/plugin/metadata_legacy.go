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
	"fmt"
	"strings"
	"unicode"
)

// Downloaders represents the plugins capability if it can retrieve
// charts from special sources
type Downloaders struct {
	// Protocols are the list of schemes from the charts URL.
	Protocols []string `yaml:"protocols"`
	// Command is the executable path with which the plugin performs
	// the actual download for the corresponding Protocols
	Command string `yaml:"command"`
}

// MetadataLegacy is the legacy plugin.yaml format
type MetadataLegacy struct {
	// Name is the name of the plugin
	Name string `yaml:"name"`

	// Version is a SemVer 2 version of the plugin.
	Version string `yaml:"version"`

	// Usage is the single-line usage text shown in help
	Usage string `yaml:"usage"`

	// Description is a long description shown in places like `helm help`
	Description string `yaml:"description"`

	// PlatformCommand is the plugin command, with a platform selector and support for args.
	PlatformCommand []PlatformCommand `yaml:"platformCommand"`

	// Command is the plugin command, as a single string.
	// DEPRECATED: Use PlatformCommand instead. Removed in subprocess/v1 plugins.
	Command string `yaml:"command"`

	// IgnoreFlags ignores any flags passed in from Helm
	IgnoreFlags bool `yaml:"ignoreFlags"`

	// PlatformHooks are commands that will run on plugin events, with a platform selector and support for args.
	PlatformHooks PlatformHooks `yaml:"platformHooks"`

	// Hooks are commands that will run on plugin events, as a single string.
	// DEPRECATED: Use PlatformHooks instead. Removed in subprocess/v1 plugins.
	Hooks Hooks `yaml:"hooks"`

	// Downloaders field is used if the plugin supply downloader mechanism
	// for special protocols.
	Downloaders []Downloaders `yaml:"downloaders"`
}

func (m *MetadataLegacy) Validate() error {
	if !validPluginName.MatchString(m.Name) {
		return fmt.Errorf("invalid plugin name %q: must contain only a-z, A-Z, 0-9, _ and -", m.Name)
	}
	m.Usage = sanitizeString(m.Usage)

	if len(m.PlatformCommand) > 0 && len(m.Command) > 0 {
		return fmt.Errorf("both platformCommand and command are set")
	}

	if len(m.PlatformHooks) > 0 && len(m.Hooks) > 0 {
		return fmt.Errorf("both platformHooks and hooks are set")
	}

	// Validate downloader plugins
	for i, downloader := range m.Downloaders {
		if downloader.Command == "" {
			return fmt.Errorf("downloader %d has empty command", i)
		}
		if len(downloader.Protocols) == 0 {
			return fmt.Errorf("downloader %d has no protocols", i)
		}
		for j, protocol := range downloader.Protocols {
			if protocol == "" {
				return fmt.Errorf("downloader %d has empty protocol at index %d", i, j)
			}
		}
	}

	return nil
}

// sanitizeString normalize spaces and removes non-printable characters.
func sanitizeString(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, str)
}
