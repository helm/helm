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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginUpdateArgs(t *testing.T) {
	o := pluginUpdateOptions{
		all: true,
	}
	assert.Nil(t, o.args([]string{}))
	assert.Error(t, o.args([]string{"foo"}))
	o = pluginUpdateOptions{
		all: false,
	}
	assert.Error(t, o.args([]string{}))
	assert.Nil(t, o.args([]string{"foo"}))
}

func TestPluginFindSelectedPlugins(t *testing.T) {
	settings.PluginsDirectory = "testdata/helmhome/helm/plugins"
	o := pluginUpdateOptions{
		all: true,
	}
	assert.Nil(t, o.findSelectedPlugins([]string{}))
	assert.Len(t, o.plugins, 5)
	var names []string
	for _, plugin := range o.plugins {
		names = append(names, plugin.Metadata.Name)
	}
	assert.ElementsMatch(t, []string{"args", "echo", "env", "exitwith", "fullenv"}, names)

	o = pluginUpdateOptions{
		all: false,
	}
	assert.Error(t, o.findSelectedPlugins([]string{"args", "foo"}))

	o = pluginUpdateOptions{
		all: false,
	}
	assert.Nil(t, o.findSelectedPlugins([]string{"args", "exitwith"}))
	names = []string{}
	for _, plugin := range o.plugins {
		names = append(names, plugin.Metadata.Name)
	}
	assert.ElementsMatch(t, []string{"args", "exitwith"}, names)
}
