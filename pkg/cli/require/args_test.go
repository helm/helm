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

package require

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestArgs(t *testing.T) {
	runTestCases(t, []testCase{{
		validateFunc: NoArgs,
	}, {
		args:         []string{"one"},
		validateFunc: NoArgs,
		wantError:    `"root" accepts no arguments`,
	}, {
		args:         []string{"one"},
		validateFunc: ExactArgs(1),
	}, {
		validateFunc: ExactArgs(1),
		wantError:    `"root" requires 1 argument`,
	}, {
		validateFunc: ExactArgs(2),
		wantError:    `"root" requires 2 arguments`,
	}, {
		args:         []string{"one"},
		validateFunc: MaximumNArgs(1),
	}, {
		args:         []string{"one", "two"},
		validateFunc: MaximumNArgs(1),
		wantError:    `"root" accepts at most 1 argument`,
	}, {
		validateFunc: MinimumNArgs(1),
		wantError:    `"root" requires at least 1 argument`,
	}, {
		args:         []string{"one", "two"},
		validateFunc: MinimumNArgs(1),
	}})
}

type testCase struct {
	args         []string
	validateFunc cobra.PositionalArgs
	wantError    string
}

func runTestCases(t *testing.T, testCases []testCase) {
	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			cmd := &cobra.Command{
				Use:  "root",
				Run:  func(*cobra.Command, []string) {},
				Args: tc.validateFunc,
			}
			cmd.SetArgs(tc.args)
			cmd.SetOutput(ioutil.Discard)

			err := cmd.Execute()
			if tc.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error, got '%v'", err)
				}
				return
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error \n\nWANT:\n%q\n\nGOT:\n%q\n", tc.wantError, err)
			}
			if !strings.Contains(err.Error(), "Usage:") {
				t.Fatalf("unexpected error: want Usage string\n\nGOT:\n%q\n", err)
			}
		})
	}
}
