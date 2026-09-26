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

package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNoDeprecations(t *testing.T) {
	deprecated := &k8sYamlStruct{
		APIVersion: "extensions/v1beta1",
		Kind:       "Deployment",
	}
	err := validateNoDeprecations(deprecated, nil)
	require.Error(t, err, "Expected deprecated extension to be flagged")
	var depErr deprecatedAPIError
	require.ErrorAs(t, err, &depErr)
	require.NotEmptyf(t, depErr.Message, "Expected error message to be non-blank")

	assert.NoError(t, validateNoDeprecations(&k8sYamlStruct{
		APIVersion: "v1",
		Kind:       "Pod",
	}, nil), "Expected a v1 Pod to not be deprecated")
}
