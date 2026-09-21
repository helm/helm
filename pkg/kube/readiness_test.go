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

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEvaluateReadiness_FailurePrecedesSuccess(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"phase": "Failed",
		},
	}}

	got, useKstatus, warnings, err := EvaluateCustomReadiness(
		obj,
		[]string{`{.phase} == Failed`},
		[]string{`{.phase} == Failed`},
	)

	require.NoError(t, err)
	assert.Equal(t, ReadinessFailed, got)
	assert.False(t, useKstatus)
	assert.Empty(t, warnings)
}
