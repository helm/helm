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

package support

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errLint = errors.New("lint failed")

func TestRunLinterRule(t *testing.T) {
	var tests = []struct {
		Severity                int
		LintError               error
		ExpectedMessages        int
		ExpectedReturn          bool
		ExpectedHighestSeverity int
	}{
		{InfoSev, errLint, 1, false, InfoSev},
		{WarningSev, errLint, 2, false, WarningSev},
		{ErrorSev, errLint, 3, false, ErrorSev},
		// No error so it returns true
		{ErrorSev, nil, 3, true, ErrorSev},
		// Retains highest severity
		{InfoSev, errLint, 4, false, ErrorSev},
		// Invalid severity values
		{4, errLint, 4, false, ErrorSev},
		{22, errLint, 4, false, ErrorSev},
		{-1, errLint, 4, false, ErrorSev},
	}

	linter := Linter{}
	for _, test := range tests {
		isValid := linter.RunLinterRule(test.Severity, "chart", test.LintError)
		assert.Lenf(t, linter.Messages, test.ExpectedMessages, "RunLinterRule(%d, \"chart\", %v), linter.Messages should now have %d message, we got %d", test.Severity, test.LintError, test.ExpectedMessages, len(linter.Messages))
		assert.Equalf(t, test.ExpectedHighestSeverity, linter.HighestSeverity, "RunLinterRule(%d, \"chart\", %v), linter.HighestSeverity should be %d, we got %d", test.Severity, test.LintError, test.ExpectedHighestSeverity, linter.HighestSeverity)
		assert.Equalf(t, test.ExpectedReturn, isValid, "RunLinterRule(%d, \"chart\", %v), should have returned %t but returned %t", test.Severity, test.LintError, test.ExpectedReturn, isValid)
	}
}

func TestMessage(t *testing.T) {
	m := Message{ErrorSev, "Chart.yaml", errors.New("Foo")}
	require.EqualError(t, m, "[ERROR] Chart.yaml: Foo")

	m = Message{WarningSev, "templates/", errors.New("Bar")}
	require.EqualError(t, m, "[WARNING] templates/: Bar")

	m = Message{InfoSev, "templates/rc.yaml", errors.New("FooBar")}
	assert.EqualError(t, m, "[INFO] templates/rc.yaml: FooBar")
}
