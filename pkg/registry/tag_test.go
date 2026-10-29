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

func TestGetTagMatchingVersionOrConstraint_LooseTagReturnedVerbatim(t *testing.T) {
	// A loose tag like v1.2.0 must come back as stored: normalizing it
	// through semver would return 1.2.0, which is not a tag in the registry.
	tags := []string{"v1.2.0", "1.0.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, ">=1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.0", got)
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

func TestGetTagMatchingVersionOrConstraint_UnderscoreTag(t *testing.T) {
	// Tags returns the original registry strings, so a chart stored under
	// the underscore convention from #10166 arrives here as 1.1.17_meta.
	// It must still match a constraint, and the returned form is the
	// plus form: the pull path converts it back to the underscore tag
	// when fetching, while user-facing uses such as the pulled file
	// name show the chart version.
	tags := []string{"1.1.17_meta", "1.0.0"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "1.x.x")
	require.NoError(t, err)
	assert.Equal(t, "1.1.17+meta", got)

	// A version written with build metadata must match the underscore tag.
	got, err = GetTagMatchingVersionOrConstraint(tags, "1.1.17+meta")
	require.NoError(t, err)
	assert.Equal(t, "1.1.17+meta", got)

	// A version written in the underscore form itself must match too. This
	// goes through the exact-match branch, which only works because Tags
	// now returns the stored tag: before, the tag arrived as 1.1.17+meta
	// and a request for 1.1.17_meta matched nothing.
	got, err = GetTagMatchingVersionOrConstraint(tags, "1.1.17_meta")
	require.NoError(t, err)
	assert.Equal(t, "1.1.17+meta", got)
}

func TestGetTagMatchingVersionOrConstraint_ExactMatchHasPrecedence(t *testing.T) {
	// Exact match should be returned even if another earlier tag would match the parsed constraint
	tags := []string{"1.3.0", "1.2.3"}
	got, err := GetTagMatchingVersionOrConstraint(tags, "1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", got, "expected exact match")
}
