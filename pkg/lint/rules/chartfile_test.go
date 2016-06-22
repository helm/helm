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

package rules

import (
	"k8s.io/helm/pkg/lint/support"
	"strings"
	"testing"
)

const badchartfile = "testdata/badchartfile"

func TestChartfile(t *testing.T) {
	linter := support.Linter{ChartDir: badchartfile}
	Chartfile(&linter)
	msgs := linter.Messages

	if len(msgs) != 3 {
		t.Errorf("Expected 3 errors, got %d", len(msgs))
	}

	if !strings.Contains(msgs[0].Text, "'name' is required") {
		t.Errorf("Unexpected message 0: %s", msgs[0].Text)
	}

	if !strings.Contains(msgs[1].Text, "'name' and directory do not match") {
		t.Errorf("Unexpected message 1: %s", msgs[1].Text)
	}

	if !strings.Contains(msgs[2].Text, "'version' 0.0.0 is less than or equal to 0") {
		t.Errorf("Unexpected message 2: %s", msgs[2].Text)
	}
}
