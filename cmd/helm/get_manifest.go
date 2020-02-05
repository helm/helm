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

	"github.com/spf13/cobra"

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/internal/completion"
	"helm.sh/helm/v3/pkg/action"
)

var getManifestHelp = `
This command fetches the generated manifest for a given release.

A manifest is a YAML-encoded representation of the Kubernetes resources that
were generated from this release's chart(s). If a chart is dependent on other
charts, those resources will also be included in the manifest.
`

func newGetManifestCmd(cfg *action.Configuration, out io.Writer) *cobra.Command {
	client := action.NewGet(cfg)

	cmd := &cobra.Command{
		Use:   "manifest RELEASE_NAME",
		Short: "download the manifest for a named release",
		Long:  getManifestHelp,
		Args:  require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := client.Run(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(out, res.Manifest)
			return nil
		},
	}

	// Function providing dynamic auto-completion
	completion.RegisterValidArgsFunc(cmd, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) != 0 {
			return nil, completion.BashCompDirectiveNoFileComp
		}
		return compListReleases(toComplete, cfg)
	})

	f := cmd.Flags()
	f.IntVar(&client.Version, "revision", 0, "get the named release with revision")
	flag := f.Lookup("revision")
	completion.RegisterFlagCompletionFunc(flag, func(cmd *cobra.Command, args []string, toComplete string) ([]string, completion.BashCompDirective) {
		if len(args) == 1 {
			return compListRevisions(cfg, args[0])
		}
		return nil, completion.BashCompDirectiveNoFileComp
	})

	return cmd
}
