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

package action

import (
	"testing"

	"github.com/stretchr/testify/require"

	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

func TestFilterManifestsToKeep_IgnoreResourcePolicy(t *testing.T) {
	manifest := `apiVersion: v1
kind: Secret
metadata:
  name: kept-secret
  annotations:
    helm.sh/resource-policy: keep
`
	_, files, err := releaseutil.SortManifests(
		releaseutil.SplitManifests(manifest), nil, releaseutil.UninstallOrder)
	require.NoError(t, err)

	t.Run("default behavior keeps the resource", func(t *testing.T) {
		kept, toDelete := filterManifestsToKeep(files, false)
		require.Len(t, kept, 1)
		require.Empty(t, toDelete)
	})

	t.Run("ignore option includes the resource for deletion", func(t *testing.T) {
		kept, toDelete := filterManifestsToKeep(files, true)
		require.Empty(t, kept)
		require.Len(t, toDelete, 1)
		require.Equal(t, "kept-secret", toDelete[0].Head.Metadata.Name)
	})
}
