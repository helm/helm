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
	"io"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/action"
)

const pushDesc = `
Upload a chart to a registry.

This command will take a distributable, compressed chart and upload it to a registry. 

The chart must already exist in the local registry cache, which can be created with either "helm package" or "helm pull".

Some registries require you to be authenticated with "helm login" before uploading a chart.
`

func newPushCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "push [...]",
		Short: "upload a chart to a registry",
		Long:  pushDesc,
		Args:  require.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.Errorf("need at least one argument: the name of the chart to push")
			}
			for i := 0; i < len(args); i++ {
				ref := args[i]
				if err := action.NewChartPush(cfg).Run(out, ref); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
