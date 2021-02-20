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
)

func TestPluginListCmd(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "List plugins",
		cmd:    "plugin list",
		golden: "output/plugin-list.txt",
	}, {
		name:   "List plugins with JSON",
		cmd:    "plugin list -o json",
		golden: "output/plugin-list-json.txt",
	}, {
		name:   "List plugins with YAML",
		cmd:    "plugin list -o yaml",
		golden: "output/plugin-list-yaml.txt",
	}}
	runTestCmd(t, tests)
}

func TestPluginListOutputCompletion(t *testing.T) {
	outputFlagCompletionTest(t, "plugin list")
}
