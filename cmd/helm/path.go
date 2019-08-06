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

	"helm.sh/helm/cmd/helm/require"
	"helm.sh/helm/pkg/helmpath"
)

var longPathHelp = `
This command displays the locations where Helm stores files.

To display a specific location, use 'helm path [config|data|cache]'.
`

var pathArgMap = map[string]string{
	"config": helmpath.ConfigPath(),
	"data":   helmpath.DataPath(),
	"cache":  helmpath.CachePath(),
}

func newPathCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "path",
		Short:   "displays the locations where Helm stores files",
		Aliases: []string{"home"},
		Long:    longPathHelp,
		Args:    require.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				if p, ok := pathArgMap[args[0]]; ok {
					fmt.Fprintln(out, p)
				} else {
					var validArgs []string
					for arg := range pathArgMap {
						validArgs = append(validArgs, arg)
					}
					return fmt.Errorf("invalid argument '%s'. Must be one of: %s", args[0], validArgs)
				}
			} else {
				// NOTE(bacongobbler): the order here is important: we want to display the config path
				// first so users can parse the first line to replicate Helm 2's `helm home`.
				fmt.Fprintln(out, helmpath.ConfigPath())
				fmt.Fprintln(out, helmpath.DataPath())
				fmt.Fprintln(out, helmpath.CachePath())
				if settings.Debug {
					fmt.Fprintf(out, "Archive: %s\n", helmpath.Archive())
					fmt.Fprintf(out, "PluginCache: %s\n", helmpath.PluginCache())
					fmt.Fprintf(out, "Plugins: %s\n", helmpath.Plugins())
					fmt.Fprintf(out, "Registry: %s\n", helmpath.Registry())
					fmt.Fprintf(out, "RepositoryCache: %s\n", helmpath.RepositoryCache())
					fmt.Fprintf(out, "RepositoryFile: %s\n", helmpath.RepositoryFile())
					fmt.Fprintf(out, "Starters: %s\n", helmpath.Starters())
				}
			}
			return nil
		},
	}
	return cmd
}
