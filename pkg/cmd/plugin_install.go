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
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/internal/plugin/installer"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
)

type pluginInstallOptions struct {
	source  string
	version string
	// OCI-specific options
	certFile              string
	keyFile               string
	caFile                string
	insecureSkipTLSverify bool
	plainHTTP             bool
	password              string
	username              string
}

const pluginInstallDesc = `
This command allows you to install a plugin from a url to a VCS repo or a local path.
`

func newPluginInstallCmd(out io.Writer) *cobra.Command {
	o := &pluginInstallOptions{}
	cmd := &cobra.Command{
		Use:     "install [options] <path|url>",
		Short:   "install a Helm plugin",
		Long:    pluginInstallDesc,
		Aliases: []string{"add"},
		Args:    require.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// We do file completion, in case the plugin is local
				return nil, cobra.ShellCompDirectiveDefault
			}
			// No more completion once the plugin path has been specified
			return noMoreArgsComp()
		},
		PreRunE: func(_ *cobra.Command, args []string) error {
			return o.complete(args)
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return o.run(out)
		},
	}
	cmd.Flags().StringVar(&o.version, "version", "", "specify a version constraint. If this is not specified, the latest version is installed")

	// Add OCI-specific flags
	cmd.Flags().StringVar(&o.certFile, "cert-file", "", "identify registry client using this SSL certificate file")
	cmd.Flags().StringVar(&o.keyFile, "key-file", "", "identify registry client using this SSL key file")
	cmd.Flags().StringVar(&o.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	cmd.Flags().BoolVar(&o.insecureSkipTLSverify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the plugin download")
	cmd.Flags().BoolVar(&o.plainHTTP, "plain-http", false, "use insecure HTTP connections for the plugin download")
	cmd.Flags().StringVar(&o.username, "username", "", "registry username")
	cmd.Flags().StringVar(&o.password, "password", "", "registry password")
	return cmd
}

func (o *pluginInstallOptions) complete(args []string) error {
	o.source = args[0]
	return nil
}

func (o *pluginInstallOptions) newInstallerForSource() (installer.Installer, error) {
	// Check if source is an OCI registry reference
	if strings.HasPrefix(o.source, fmt.Sprintf("%s://", registry.OCIScheme)) {
		// Build getter options for OCI
		options := []getter.Option{
			getter.WithTLSClientConfig(o.certFile, o.keyFile, o.caFile),
			getter.WithInsecureSkipVerifyTLS(o.insecureSkipTLSverify),
			getter.WithPlainHTTP(o.plainHTTP),
			getter.WithBasicAuth(o.username, o.password),
		}

		return installer.NewOCIInstaller(o.source, options...)
	}

	// For non-OCI sources, use the original logic
	return installer.NewForSource(o.source, o.version)
}

func (o *pluginInstallOptions) run(out io.Writer) error {
	installer.Debug = settings.Debug

	i, err := o.newInstallerForSource()
	if err != nil {
		return err
	}
	if err := installer.Install(i); err != nil {
		return err
	}

	slog.Debug("loading plugin", "path", i.Path())
	p, err := plugin.LoadDir(i.Path())
	if err != nil {
		return fmt.Errorf("plugin is installed but unusable: %w", err)
	}

	if err := runHook(p, plugin.Install); err != nil {
		return err
	}

	fmt.Fprintf(out, "Installed plugin: %s\n", p.Metadata().Name)
	return nil
}
