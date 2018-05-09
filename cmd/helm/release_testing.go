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

package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/helm/pkg/hapi/release"
	"k8s.io/helm/pkg/helm"
)

const releaseTestDesc = `
The test command runs the tests for a release.

The argument this command takes is the name of a deployed release.
The tests to be run are defined in the chart that was installed.
`

type releaseTestOptions struct {
	name    string
	client  helm.Interface
	timeout int64
	cleanup bool
}

func newReleaseTestCmd(c helm.Interface, out io.Writer) *cobra.Command {
	o := &releaseTestOptions{client: c}

	cmd := &cobra.Command{
		Use:   "test [RELEASE]",
		Short: "test a release",
		Long:  releaseTestDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkArgsLength(len(args), "release name"); err != nil {
				return err
			}

			o.name = args[0]
			o.client = ensureHelmClient(o.client, false)
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.Int64Var(&o.timeout, "timeout", 300, "time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks)")
	f.BoolVar(&o.cleanup, "cleanup", false, "delete test pods upon completion")

	return cmd
}

func (o *releaseTestOptions) run(out io.Writer) (err error) {
	c, errc := o.client.RunReleaseTest(
		o.name,
		helm.ReleaseTestTimeout(o.timeout),
		helm.ReleaseTestCleanup(o.cleanup),
	)
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
}

type testErr struct {
	failed int
}

func (err *testErr) Error() error {
	return fmt.Errorf("%v test(s) failed", err.failed)
}
