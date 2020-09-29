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

	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/plugin"
	"helm.sh/helm/v3/pkg/plugin/installer"
)

type pluginInstallOptions struct {
	source  string
	version string
}

const pluginInstallDesc = `
This command allows you to install a plugin from a url to a VCS repo or a local path.
`

func newPluginInstallCmd(out io.Writer) *cobra.Command {
	o := &pluginInstallOptions{}
	cmd := &cobra.Command{
		Use:     "install [options] <path|url>...",
		Short:   "install one or more Helm plugins",
		Long:    pluginInstallDesc,
		Aliases: []string{"add"},
		Args:    require.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// We do file completion, in case the plugin is local
				return nil, cobra.ShellCompDirectiveDefault
			}
			// No more completion once the plugin path has been specified
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return o.complete(args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.run(out)
		},
	}
	cmd.Flags().StringVar(&o.version, "version", "", "specify a version constraint. If this is not specified, the latest version is installed")
	return cmd
}

func (o *pluginInstallOptions) complete(args []string) error {
	o.source = args[0]
	return nil
}

func (o *pluginInstallOptions) run(out io.Writer) error {
	installer.Debug = settings.Debug

	i, err := installer.NewForSource(o.source, o.version)
	if err != nil {
		return err
	}
	if err := installer.Install(i); err != nil {
		return err
	}

	debug("loading plugin from %s", i.Path())
	p, err := plugin.LoadDir(i.Path())
	if err != nil {
		return errors.Wrap(err, "plugin is installed but unusable")
	}

	if err := runHook(p, plugin.Install); err != nil {
		return err
	}

	fmt.Fprintf(out, "Installed plugin: %s\n", p.Metadata.Name)
	return nil
}
