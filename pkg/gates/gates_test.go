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

package gates

import (
	"os"
	"testing"
)

const name string = "HELM_EXPERIMENTAL_FEATURE"

func TestIsEnabled(t *testing.T) {
	os.Unsetenv(name)
	g := Gate(name)

	if g.IsEnabled() {
		t.Errorf("feature gate shows as available, but the environment variable %s was not set", name)
	}

	os.Setenv(name, "1")

	if !g.IsEnabled() {
		t.Errorf("feature gate shows as disabled, but the environment variable %s was set", name)
	}
}

func TestError(t *testing.T) {
	os.Unsetenv(name)
	g := Gate(name)

	if g.Error().Error() != "this feature has been marked as experimental and is not enabled by default. Please set HELM_EXPERIMENTAL_FEATURE=1 in your environment to use this feature" {
		t.Errorf("incorrect error message. Received %s", g.Error().Error())
	}
}

func TestString(t *testing.T) {
	os.Unsetenv(name)
	g := Gate(name)

	if g.String() != "HELM_EXPERIMENTAL_FEATURE" {
		t.Errorf("incorrect string representation. Received %s", g.String())
	}
}
