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

	"k8s.io/helm/pkg/action"
)

const chartListDesc = `
TODO
`

func newChartListCmd(cfg *action.Configuration) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list all saved charts",
		Long:    chartListDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return action.NewChartList(cfg).Run()
		},
	}
}
