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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTagMatchingVersionOrConstraint_ExactMatch(t *testing.T) {
	tags := []string{"1.0.0", "1.2.3", "2.0.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "1.2.3")
	require.NoError(t, err)
	require.Equal(t, "1.2.3", got, "expected exact match")
}

func TestGetTagMatchingVersionOrConstraint_EmptyVersionWildcard(t *testing.T) {
	// Includes a non-semver tag which should be skipped
	tags := []string{"latest", "0.9.0", "1.0.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "")
	require.NoError(t, err)
	// Should pick the first valid semver tag in order, which is 0.9.0
	require.Equal(t, "0.9.0", got)
}

func TestGetTagMatchingVersionOrConstraint_ConstraintRange(t *testing.T) {
	tags := []string{"0.5.0", "1.0.0", "1.1.0", "2.0.0"}

	// Caret range
	got, err := GetTagMatchingVersionOrConstraint(tags, "^1.0.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", got, "first match in order")

	// Compound range
	got, err = GetTagMatchingVersionOrConstraint(tags, ">=1.0.0 <2.0.0")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", got)
}

func TestGetTagMatchingVersionOrConstraint_InvalidConstraint(t *testing.T) {
	tags := []string{"1.0.0"}
	_, err := GetTagMatchingVersionOrConstraint(tags, ">a1")
	require.Error(t, err, "expected error for invalid constraint")
}

func TestGetTagMatchingVersionOrConstraint_NoMatches(t *testing.T) {
	tags := []string{"0.1.0", "0.2.0"}
	_, err := GetTagMatchingVersionOrConstraint(tags, ">=1.0.0")
	assert.ErrorContains(t, err, ">=1.0.0", "expected error to contain version string")
}

func TestGetTagMatchingVersionOrConstraint_SkipsNonSemverTags(t *testing.T) {
	tags := []string{"alpha", "1.0.0", "beta", "1.1.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, ">=1.0.0 <2.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", got)
}

func TestGetTagMatchingVersionOrConstraint_OrderMatters_FirstMatchReturned(t *testing.T) {
	// Both 1.2.0 and 1.3.0 satisfy >=1.2.0 <2.0.0, but the function returns the first in input order
	tags := []string{"1.3.0", "1.2.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, ">=1.2.0 <2.0.0")
	require.NoError(t, err)
	assert.Equal(t, "1.3.0", got, "first satisfying tag")
}

func TestGetTagMatchingVersionOrConstraint_ExactMatchHasPrecedence(t *testing.T) {
	// Exact match should be returned even if another earlier tag would match the parsed constraint
	tags := []string{"1.3.0", "1.2.3"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", got, "expected exact match")
}
