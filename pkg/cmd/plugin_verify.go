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
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/pkg/cmd/require"
)

const pluginVerifyDesc = `
This command verifies that a Helm plugin has a valid provenance file,
and that the provenance file is signed by a trusted PGP key.

It supports both:
- Plugin tarballs (.tgz or .tar.gz files)
- Installed plugin directories

For installed plugins, use the path shown by 'helm env HELM_PLUGINS' followed
by the plugin name. For example:
  helm plugin verify ~/.local/share/helm/plugins/example-cli

To generate a signed plugin, use the 'helm plugin package --sign' command.
`

type pluginVerifyOptions struct {
	keyring    string
	pluginPath string
}

func newPluginVerifyCmd(out io.Writer) *cobra.Command {
	o := &pluginVerifyOptions{}

	cmd := &cobra.Command{
		Use:   "verify [PATH]",
		Short: "verify that a plugin at the given path has been signed and is valid",
		Long:  pluginVerifyDesc,
		Args:  require.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			o.pluginPath = args[0]
			return o.run(out)
		},
	}

	cmd.Flags().StringVar(&o.keyring, "keyring", defaultKeyring(), "keyring containing public keys")

	return cmd
}

func (o *pluginVerifyOptions) run(out io.Writer) error {
	// Verify the plugin path exists
	fi, err := os.Stat(o.pluginPath)
	if err != nil {
		return err
	}

	// Only support tarball verification
	if fi.IsDir() {
		return fmt.Errorf("directory verification not supported - only plugin tarballs can be verified")
	}

	// Verify it's a tarball
	if !plugin.IsTarball(o.pluginPath) {
		return fmt.Errorf("plugin file must be a gzipped tarball (.tar.gz or .tgz)")
	}

	// Look for provenance file
	provFile := o.pluginPath + ".prov"
	if _, err := os.Stat(provFile); err != nil {
		return fmt.Errorf("could not find provenance file %s: %w", provFile, err)
	}

	// Read the files
	archiveData, err := os.ReadFile(o.pluginPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin file: %w", err)
	}

	provData, err := os.ReadFile(provFile)
	if err != nil {
		return fmt.Errorf("failed to read provenance file: %w", err)
	}

	// Verify the plugin using data
	verification, err := plugin.VerifyPlugin(archiveData, provData, filepath.Base(o.pluginPath), o.keyring)
	if err != nil {
		return err
	}

	// Output verification details
	for name := range verification.SignedBy.Identities {
		fmt.Fprintf(out, "Signed by: %v\n", name)
	}
	fmt.Fprintf(out, "Using Key With Fingerprint: %X\n", verification.SignedBy.PrimaryKey.Fingerprint)

	// Only show hash for tarballs
	if verification.FileHash != "" {
		fmt.Fprintf(out, "Plugin Hash Verified: %s\n", verification.FileHash)
	} else {
		fmt.Fprintf(out, "Plugin Metadata Verified: %s\n", verification.FileName)
	}

	return nil
}
