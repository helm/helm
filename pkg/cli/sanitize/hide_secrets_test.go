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

	"helm.sh/helm/v3/pkg/release"
)

func TestHideManifestSecrets(t *testing.T) {

	for _, testCase := range []struct {
		description   string
		manifestFile  string
		sanitizedFile string
	}{
		{
			description:   "replace manifest with sanitized one",
			manifestFile:  "testdata/manifest-input.yaml",
			sanitizedFile: "testdata/manifest-sanitized.yaml",
		},
		{
			description:   "handle different secrets",
			manifestFile:  "testdata/different-secrets.yaml",
			sanitizedFile: "testdata/different-secrets-sanitized.yaml",
		},
		{
			description:   "do not modify manifest when no secret values",
			manifestFile:  "testdata/manifest-no-secret.yaml",
			sanitizedFile: "testdata/manifest-no-secret.yaml",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			manifestRaw, err := ioutil.ReadFile(testCase.manifestFile)
			require.NoError(t, err)
			expectedManifestRaw, err := ioutil.ReadFile(testCase.sanitizedFile)
			require.NoError(t, err)

			rel := &release.Release{
				Name:     "test",
				Manifest: string(manifestRaw),
			}

			err = HideManifestSecrets(rel)
			require.NoError(t, err)

			assert.Equal(t, string(expectedManifestRaw), rel.Manifest)
		})
	}

	t.Run("ignore nil release", func(t *testing.T) {
		var rel *release.Release

		err := HideManifestSecrets(rel)
		require.NoError(t, err)
		assert.Nil(t, rel)
	})

	t.Run("include secret name in error message", func(t *testing.T) {
		manifestRaw, err := ioutil.ReadFile("testdata/invalid-secret.yaml")
		require.NoError(t, err)

		rel := &release.Release{
			Name:     "test",
			Manifest: string(manifestRaw),
		}
		err = HideManifestSecrets(rel)
		require.Error(t, err)
		require.Contains(t, err.Error(), "\"invalid-secret-with-dup-values\"")
	})
}
