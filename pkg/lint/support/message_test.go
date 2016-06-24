/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"fmt"
	"testing"
)

var linter Linter = Linter{}
var lintError LintError = fmt.Errorf("Foobar")

func TestRunLinterRule(t *testing.T) {
	var tests = []struct {
		Severity         int
		LintError        error
		ExpectedMessages int
		ExpectedReturn   bool
	}{
		{ErrorSev, lintError, 1, false},
		{WarningSev, lintError, 2, false},
		{InfoSev, lintError, 3, false},
		// No error so it returns true
		{ErrorSev, nil, 3, true},
		// Invalid severity values
		{4, lintError, 3, false},
		{22, lintError, 3, false},
		{-1, lintError, 3, false},
	}

	for _, test := range tests {
		isValid := linter.RunLinterRule(test.Severity, test.LintError)
		if len(linter.Messages) != test.ExpectedMessages {
			t.Errorf("RunLinterRule(%d, %v), linter.Messages should have now %d message, we got %d", test.Severity, test.LintError, test.ExpectedMessages, len(linter.Messages))
		}

		if isValid != test.ExpectedReturn {
			t.Errorf("RunLinterRule(%d, %v), should have returned %t but returned %t", test.Severity, test.LintError, test.ExpectedReturn, isValid)
		}
	}
}

func TestMessage(t *testing.T) {
	m := Message{ErrorSev, "Foo"}
	if m.String() != "[ERROR] Foo" {
		t.Errorf("Unexpected output: %s", m.String())
	}

	m = Message{WarningSev, "Bar"}
	if m.String() != "[WARNING] Bar" {
		t.Errorf("Unexpected output: %s", m.String())
	}

	m = Message{InfoSev, "FooBar"}
	if m.String() != "[INFO] FooBar" {
		t.Errorf("Unexpected output: %s", m.String())
	}
}
