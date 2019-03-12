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
	"io"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/release"
)

const releaseTestDesc = `
The test command runs the tests for a release.

The argument this command takes is the name of a deployed release.
The tests to be run are defined in the chart that was installed.
`

func newReleaseTestCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewReleaseTesting(cfg)

	cmd := &cobra.Command{
		Use:   "test [RELEASE]",
		Short: "test a release",
		Long:  releaseTestDesc,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, errc := client.Run(args[0])
			testErr := &testErr{}

			for {
				select {
				case err := <-errc:
					if err == nil && testErr.failed > 0 {
						return testErr.Error()
					}
					return err
				case res, ok := <-c:
					if !ok {
						break
					}

					if res.Status == release.TestRunFailure {
						testErr.failed++
					}
					fmt.Fprintf(out, res.Msg+"\n")
				}
			}
		},
	}

	f := cmd.Flags()
	f.Int64Var(&client.Timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&client.Cleanup, "cleanup", false, "delete test pods upon completion")

	return cmd
}

type testErr struct {
	failed int
}

func (err *testErr) Error() error {
	return errors.Errorf("%v test(s) failed", err.failed)
}
