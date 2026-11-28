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

package sequence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStoredManifests(t *testing.T) {
	t.Parallel()

	t.Run("recovers source paths and preserves positional fallbacks", func(t *testing.T) {
		stream := "---\n# Source: parent/templates/a.yaml\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n" +
			"---\n# Source: parent/charts/db/templates/b.yaml\napiVersion: v1\nkind: Service\nmetadata:\n  name: b\n" +
			"---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: c\n"

		manifests, err := ParseStoredManifests(stream)
		require.NoError(t, err)
		require.Len(t, manifests, 3)
		assert.Equal(t, "parent/templates/a.yaml", manifests[0].Name)
		assert.Equal(t, "parent/charts/db/templates/b.yaml", manifests[1].Name)
		assert.Equal(t, "manifest-2", manifests[2].Name)
		assert.Equal(t, "ConfigMap", manifests[0].Head.Kind)
		assert.Equal(t, "a", manifests[0].Head.Metadata.Name)
		assert.Equal(t, "Service", manifests[1].Head.Kind)
		assert.Equal(t, "Secret", manifests[2].Head.Kind)
	})

	t.Run("reports malformed YAML against its recovered source path", func(t *testing.T) {
		_, err := ParseStoredManifests("---\n# Source: parent/templates/bad.yaml\nkind: ConfigMap\nmetadata:\n  name: [\n")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "YAML parse error on parent/templates/bad.yaml")
	})
}
