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

	"github.com/stretchr/testify/assert"

	release "helm.sh/helm/v4/pkg/release/v1"
)

func TestGetMetadata_Labels(t *testing.T) {
	rel := releaseStub()
	rel.Info.Status = release.StatusDeployed
	customLabels := map[string]string{"key1": "value1", "key2": "value2"}
	rel.Labels = customLabels

	metaGetter := NewGetMetadata(actionConfigFixture(t))
	err := metaGetter.cfg.Releases.Create(rel)
	assert.NoError(t, err)

	metadata, err := metaGetter.Run(rel.Name)
	assert.NoError(t, err)

	assert.Equal(t, metadata.Name, rel.Name)
	assert.Equal(t, metadata.Labels, customLabels)
}
