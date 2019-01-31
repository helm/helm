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
	"github.com/spf13/cobra"

	"k8s.io/helm/cmd/helm/require"
	"k8s.io/helm/pkg/action"
)

const chartPushDesc = `
TODO
`

func newChartPushCmd(cfg *action.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:   "push [ref]",
		Short: "push a chart to remote",
		Long:  chartPushDesc,
		Args:  require.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			return action.NewChartPush(cfg).Run(ref)
		},
	}
}
