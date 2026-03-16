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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataLegacyValidate(t *testing.T) {
	testsValid := map[string]MetadataLegacy{
		"valid metadata": {
			Name: "myplugin",
		},
		"valid with command": {
			Name:    "myplugin",
			Command: "echo hello",
		},
		"valid with platformCommand": {
			Name: "myplugin",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "linux", Architecture: "amd64", Command: "echo hello"},
			},
		},
		"valid with hooks": {
			Name: "myplugin",
			Hooks: Hooks{
				"install": "echo install",
			},
		},
		"valid with platformHooks": {
			Name: "myplugin",
			PlatformHooks: PlatformHooks{
				"install": []PlatformCommand{
					{OperatingSystem: "linux", Architecture: "amd64", Command: "echo install"},
				},
			},
		},
		"valid with downloaders": {
			Name: "myplugin",
			Downloaders: []Downloaders{
				{
					Protocols: []string{"myproto"},
					Command:   "echo download",
				},
			},
		},
	}

	for testName, metadata := range testsValid {
		t.Run(testName, func(t *testing.T) {
			assert.NoError(t, metadata.Validate())
		})
	}

	testsInvalid := map[string]MetadataLegacy{
		"invalid name": {
			Name: "my plugin", // further tested in TestValidPluginName
		},
		"both command and platformCommand": {
			Name:    "myplugin",
			Command: "echo hello",
			PlatformCommand: []PlatformCommand{
				{OperatingSystem: "linux", Architecture: "amd64", Command: "echo hello"},
			},
		},
		"both hooks and platformHooks": {
			Name: "myplugin",
			Hooks: Hooks{
				"install": "echo install",
			},
			PlatformHooks: PlatformHooks{
				"install": []PlatformCommand{
					{OperatingSystem: "linux", Architecture: "amd64", Command: "echo install"},
				},
			},
		},
		"downloader with empty command": {
			Name: "myplugin",
			Downloaders: []Downloaders{
				{
					Protocols: []string{"myproto"},
					Command:   "",
				},
			},
		},
		"downloader with no protocols": {
			Name: "myplugin",
			Downloaders: []Downloaders{
				{
					Protocols: []string{},
					Command:   "echo download",
				},
			},
		},
		"downloader with empty protocol": {
			Name: "myplugin",
			Downloaders: []Downloaders{
				{
					Protocols: []string{""},
					Command:   "echo download",
				},
			},
		},
	}

	for testName, metadata := range testsInvalid {
		t.Run(testName, func(t *testing.T) {
			assert.Error(t, metadata.Validate())
		})
	}
}
