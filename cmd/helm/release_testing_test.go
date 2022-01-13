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

package main

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"helm.sh/helm/v3/internal/test"
	"helm.sh/helm/v3/pkg/release"
	helmtime "helm.sh/helm/v3/pkg/time"
)

const (
	tableLinePattern  = `^Last (Completed|Started):\s+(.+)$`
	verbPosition      = 1 // in the line patterns
	timestampPosition = 2 // in the line patterns
)

type outputFormat struct {
	linePattern regexp.Regexp
	checkTime   func(raw string) error
}

func TestReleaseTesting(t *testing.T) {
	mockReleases := []*release.Release{
		createMockRelease(),
	}

	tableOutput := outputFormat{
		linePattern: *regexp.MustCompile(tableLinePattern),
		checkTime: func(raw string) error {
			_, err := helmtime.Parse(time.ANSIC, raw) // Layout/format must be the one actually used in the command output.
			return err
		},
	}

	tests := []cmdTestCase{
		{
			name:   "test without logs",
			cmd:    "test doge",
			golden: "output/test-without-logs.txt",
			rels:   mockReleases,
		},
		{
			name:   "test with logs",
			cmd:    "test doge --logs",
			golden: "output/test-with-logs.txt",
			rels:   mockReleases,
		},
	}

	runTestCmdWithCustomAssertion(t, tests, test.AssertGoldenStringWithCustomLineValidation(t, checkLineAs(tableOutput)))
}

func checkLineAs(out outputFormat) func(expected, actual string) (bool, error) {
	return func(expected, actual string) (bool, error) {
		expectedMatch := out.linePattern.FindStringSubmatch(expected)
		if expectedMatch != nil {
			maybeTimestamp := expectedMatch[timestampPosition]
			if out.checkTime(maybeTimestamp) == nil {
				// This line requires special treatment.
				actualMatch := out.linePattern.FindStringSubmatch(actual)
				if actualMatch == nil {
					return true, fmt.Errorf("expected to match %v", out.linePattern)
				}
				expectedVerb := expectedMatch[verbPosition]
				actualVerb := actualMatch[verbPosition]
				if actualVerb != expectedVerb {
					return true, fmt.Errorf("expected '%s', but found '%s'", expectedVerb, actualVerb)
				}
				actualTimestamp := actualMatch[timestampPosition]
				if err := out.checkTime(actualTimestamp); err != nil {
					return true, fmt.Errorf("expected timestamp of same format, but found '%s' (%s)", actualTimestamp, err.Error())
				}
				return true, nil // The actual line was identical to the expected one, modulo the point in time represented by the timestamp.
			}
		}
		// This line does not require special treatment.
		return false, nil
	}
}

func createMockRelease() *release.Release {
	rel := release.Mock(&release.MockReleaseOptions{Name: "doge"})
	rel.Hooks[0] = &release.Hook{
		Name:   "doge-test-pod",
		Kind:   "Pod",
		Path:   "doge-test-pod",
		Events: []release.HookEvent{release.HookTest},
	}
	return rel
}

func TestReleaseTestingCompletion(t *testing.T) {
	checkReleaseCompletion(t, "test", false)
}

func TestReleaseTestingFileCompletion(t *testing.T) {
	checkFileCompletion(t, "test", false)
	checkFileCompletion(t, "test myrelease", false)
}
