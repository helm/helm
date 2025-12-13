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
	// signing options
	verify  bool
	keyring string
	// OCI-specific options
	certFile              string
	keyFile               string
	caFile                string
	insecureSkipTLSVerify bool
	plainHTTP             bool
	password              string
	username              string
}

const pluginInstallDesc = `
This command allows you to install a plugin from a url to a VCS repo or a local path.

By default, plugin signatures are verified before installation when installing from
tarballs (.tgz or .tar.gz). This requires a corresponding .prov file to be available
alongside the tarball.
For local development, plugins installed from local directories are automatically
treated as "local dev" and do not require signatures.
Use --verify=false to skip signature verification for remote plugins.
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
	cmd.Flags().BoolVar(&o.verify, "verify", true, "verify the plugin signature before installing")
	cmd.Flags().StringVar(&o.keyring, "keyring", defaultKeyring(), "location of public keys used for verification")

	// Add OCI-specific flags
	cmd.Flags().StringVar(&o.certFile, "cert-file", "", "identify registry client using this SSL certificate file")
	cmd.Flags().StringVar(&o.keyFile, "key-file", "", "identify registry client using this SSL key file")
	cmd.Flags().StringVar(&o.caFile, "ca-file", "", "verify certificates of HTTPS-enabled servers using this CA bundle")
	cmd.Flags().BoolVar(&o.insecureSkipTLSVerify, "insecure-skip-tls-verify", false, "skip tls certificate checks for the plugin download")
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
			getter.WithInsecureSkipVerifyTLS(o.insecureSkipTLSVerify),
			getter.WithPlainHTTP(o.plainHTTP),
			getter.WithBasicAuth(o.username, o.password),
		}

		return installer.NewOCIInstaller(o.source, options...)
	}

	// For non-OCI sources, use the original logic
	return installer.NewForSource(o.source, o.version)
}

func (o *pluginInstallOptions) run(out io.Writer) error {
	i, err := o.newInstallerForSource()
	if err != nil {
		return err
	}

	// Determine if we should verify based on installer type and flags
	shouldVerify := o.verify

	// Check if this is a local directory installation (for development)
	if localInst, ok := i.(*installer.LocalInstaller); ok && !localInst.SupportsVerification() {
		// Local directory installations are allowed without verification
		shouldVerify = false
		fmt.Fprintf(out, "Installing plugin from local directory (development mode)\n")
	} else if shouldVerify {
		// For remote installations, check if verification is supported
		if verifier, ok := i.(installer.Verifier); !ok || !verifier.SupportsVerification() {
			return fmt.Errorf("plugin source does not support verification. Use --verify=false to skip verification")
		}
	} else {
		// User explicitly disabled verification
		fmt.Fprintf(out, "WARNING: Skipping plugin signature verification\n")
	}

	// Set up installation options
	opts := installer.Options{
		Verify:  shouldVerify,
		Keyring: o.keyring,
	}

	// If verify is requested, show verification output
	if shouldVerify {
		fmt.Fprintf(out, "Verifying plugin signature...\n")
	}

	// Install the plugin with options
	verifyResult, err := installer.InstallWithOptions(i, opts)
	if err != nil {
		return err
	}

	// If verification was successful, show the details
	if verifyResult != nil {
		for _, signer := range verifyResult.SignedBy {
			fmt.Fprintf(out, "Signed by: %s\n", signer)
		}
		fmt.Fprintf(out, "Using Key With Fingerprint: %s\n", verifyResult.Fingerprint)
		fmt.Fprintf(out, "Plugin Hash Verified: %s\n", verifyResult.FileHash)
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
