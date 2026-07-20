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

package cmd

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cmd/require"
)

const bumpDesc = `
Bump the version of a chart.
`

type bumpOptions struct {
	bumpType string
	chart    string
}

// newBumpCmd creates a new bump command with the given configuration and output writer.
func newBumpCmd(actionConfig *action.Configuration, out io.Writer) *cobra.Command {
	o := &bumpOptions{}

	cmd := &cobra.Command{
		Use:               "bump [VERSION_TYPE] [CHART]",
		Short:             "bump the version of a chart",
		Long:              bumpDesc,
		Args:              require.MaximumNArgs(2),
		ValidArgsFunction: noMoreArgsCompFunc,
		RunE: func(_ *cobra.Command, args []string) error {
			switch {
			case len(args) == 2:
				o.bumpType = args[0]
				o.chart = args[1]
			case len(args) == 1:
				o.bumpType = ""
				o.chart = args[0]
			default:
				return fmt.Errorf("invalid arguments: %v", args)
			}

			return o.run(actionConfig, out)
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.chart, "chart", "", "path to the chart directory")

	return cmd
}

func (o *bumpOptions) run(actionConfig *action.Configuration, out io.Writer) error {
	// Resolve the chart path to an absolute path
	absPath, err := filepath.Abs(o.chart)
	if err != nil {
		return fmt.Errorf("failed to resolve chart path: %w", err)
	}
	o.chart = absPath

	bump := action.NewBump(actionConfig)

	newVersion, err := bump.Run(o.bumpType, o.chart)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "changed chart version to %q\n", newVersion)
	return nil
}
