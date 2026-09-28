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

	"github.com/stretchr/testify/assert"
)

const name string = "HELM_EXPERIMENTAL_FEATURE"

func TestIsEnabled(t *testing.T) {
	g := Gate(name)

	assert.False(t, g.IsEnabled(), "feature gate shows as available, but the environment variable %s was not set", name)

	t.Setenv(name, "1")

	assert.True(t, g.IsEnabled(), "feature gate shows as disabled, but the environment variable %s was set", name)
}

func TestError(t *testing.T) {
	os.Unsetenv(name)
	g := Gate(name)

	assert.EqualError(t, g.Error(), "this feature has been marked as experimental and is not enabled by default. Please set HELM_EXPERIMENTAL_FEATURE=1 in your environment to use this feature")
}

func TestString(t *testing.T) {
	os.Unsetenv(name)
	g := Gate(name)

	assert.Equal(t, "HELM_EXPERIMENTAL_FEATURE", g.String())
}
