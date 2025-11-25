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

	"helm.sh/helm/v4/pkg/chart/v2/lint/support"
)

const invalidCrdsDir = "./testdata/invalidcrdsdir"
const emptyCrdDir = "./testdata/emptycrd"

func TestInvalidCrdsDir(t *testing.T) {
	linter := support.Linter{ChartDir: invalidCrdsDir}
	Crds(&linter)
	res := linter.Messages

	assert.Len(t, res, 1)
	assert.ErrorContains(t, res[0].Err, "not a directory")
}

// TestEmptyCrd tests that empty CRD files (containing only comments/whitespace)
// don't cause a panic when the YAML decoder returns nil.
// This is a regression test for issue #31571.
func TestEmptyCrd(t *testing.T) {
	linter := support.Linter{ChartDir: emptyCrdDir}
	Crds(&linter)
	res := linter.Messages

	// Should not panic and should have no errors
	assert.Len(t, res, 0)
}
