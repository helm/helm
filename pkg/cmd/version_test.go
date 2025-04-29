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

package cmd

import (
	"testing"
)

func TestVersion(t *testing.T) {
	tests := []cmdTestCase{{
		name:   "default",
		cmd:    "version",
		golden: "output/version.txt",
	}, {
		name:   "short",
		cmd:    "version --short",
		golden: "output/version-short.txt",
	}, {
		name:   "template",
		cmd:    "version --template='Version: {{.Version}}'",
		golden: "output/version-template.txt",
	}, {
		name:   "client",
		cmd:    "version --client",
		golden: "output/version-client.txt",
	}, {
		name:   "client shorthand",
		cmd:    "version -c",
		golden: "output/version-client-shorthand.txt",
	}}
	runTestCmd(t, tests)
}

func TestVersionFileCompletion(t *testing.T) {
	checkFileCompletion(t, "version", false)
}
