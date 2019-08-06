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
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/pkg/action"
	"helm.sh/helm/pkg/cli/values"
)

var longLintHelp = `
This command takes a path to a chart and runs a series of tests to verify that
the chart is well-formed.

If the linter encounters things that will cause the chart to fail installation,
it will emit [ERROR] messages. If it encounters issues that break with convention
or recommendation, it will emit [WARNING] messages.
`

func newLintCmd(out io.Writer) *cobra.Command {
	client := action.NewLint()
	valueOpts := &values.Options{}

	cmd := &cobra.Command{
		Use:   "lint PATH",
		Short: "examines a chart for possible issues",
		Long:  longLintHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := []string{"."}
			if len(args) > 0 {
				paths = args
			}
			client.Namespace = getNamespace()
			vals, err := valueOpts.MergeValues(settings)
			if err != nil {
				return err
			}
			result := client.Run(paths, vals)
			var message strings.Builder
			fmt.Fprintf(&message, "%d chart(s) linted, %d chart(s) failed\n", result.TotalChartsLinted, len(result.Errors))
			for _, err := range result.Errors {
				fmt.Fprintf(&message, "\t%s\n", err)
			}
			for _, msg := range result.Messages {
				fmt.Fprintf(&message, "\t%s\n", msg)
			}

			if len(result.Errors) > 0 {
				return errors.New(message.String())
			}
			fmt.Fprintf(out, message.String())
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&client.Strict, "strict", false, "fail on lint warnings")
	addValueOptionsFlags(f, valueOpts)

	return cmd
}
