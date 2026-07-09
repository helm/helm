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

package rules // import "helm.sh/helm/v4/internal/chart/v3/lint/rules"

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
	var depErr deprecatedAPIError
	require.Error(t, err, "Expected deprecated extension to be flagged")
	require.ErrorAs(t, err, &depErr, "Expected error to be of type deprecatedAPIError")
	require.NotEmpty(t, depErr.Message, "Expected error message to be non-blank: %v", err)

	err = validateNoDeprecations(&k8sYamlStruct{
		APIVersion: "v1",
		Kind:       "Pod",
	}, nil)
	assert.NoError(t, err, "Expected a v1 Pod to not be deprecated")
}
