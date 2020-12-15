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

package sanitize

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHideSecrets(t *testing.T) {

	t.Run("hide secret values", func(t *testing.T) {
		manifestRaw, err := ioutil.ReadFile("testdata/manifest-input.yaml")
		require.NoError(t, err)
		expectedManifestRaw, err := ioutil.ReadFile("testdata/manifest-sanitized.yaml")
		require.NoError(t, err)

		sanitizedManifest, err := HideSecrets(string(manifestRaw))
		require.NoError(t, err)

		assert.Equal(t, string(expectedManifestRaw), sanitizedManifest)
	})

	t.Run("do not modify, when no secret values", func(t *testing.T) {
		manifestRaw, err := ioutil.ReadFile("testdata/manifest-no-secret.yaml")
		require.NoError(t, err)

		sanitizedManifest, err := HideSecrets(string(manifestRaw))
		require.NoError(t, err)

		assert.Equal(t, string(manifestRaw), sanitizedManifest)
	})
}
