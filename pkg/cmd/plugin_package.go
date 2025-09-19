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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/pkg/cmd/require"
	"helm.sh/helm/v4/pkg/provenance"
)

const pluginPackageDesc = `
This command packages a Helm plugin directory into a tarball.

By default, the command will generate a provenance file signed with a PGP key.
This ensures the plugin can be verified after installation.

Use --sign=false to skip signing (not recommended for distribution).
`

type pluginPackageOptions struct {
	sign           bool
	keyring        string
	key            string
	passphraseFile string
	pluginPath     string
	destination    string
}

func newPluginPackageCmd(out io.Writer) *cobra.Command {
	o := &pluginPackageOptions{}

	cmd := &cobra.Command{
		Use:   "package [PATH]",
		Short: "package a plugin directory into a plugin archive",
		Long:  pluginPackageDesc,
		Args:  require.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			o.pluginPath = args[0]
			return o.run(out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.sign, "sign", true, "use a PGP private key to sign this plugin")
	f.StringVar(&o.key, "key", "", "name of the key to use when signing. Used if --sign is true")
	f.StringVar(&o.keyring, "keyring", defaultKeyring(), "location of a public keyring")
	f.StringVar(&o.passphraseFile, "passphrase-file", "", "location of a file which contains the passphrase for the signing key. Use \"-\" to read from stdin.")
	f.StringVarP(&o.destination, "destination", "d", ".", "location to write the plugin tarball.")

	return cmd
}

func (o *pluginPackageOptions) run(out io.Writer) error {
	// Check if the plugin path exists and is a directory
	fi, err := os.Stat(o.pluginPath)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("plugin package only supports directories, not tarballs")
	}

	// Load and validate plugin metadata
	pluginMeta, err := plugin.LoadDir(o.pluginPath)
	if err != nil {
		return fmt.Errorf("invalid plugin directory: %w", err)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(o.destination, 0755); err != nil {
		return err
	}

	// If signing is requested, prepare the signer first
	var signer *provenance.Signatory
	if o.sign {
		// Load the signing key
		signer, err = provenance.NewFromKeyring(o.keyring, o.key)
		if err != nil {
			return fmt.Errorf("error reading from keyring: %w", err)
		}

		// Get passphrase
		passphraseFetcher := o.promptUser
		if o.passphraseFile != "" {
			passphraseFetcher, err = o.passphraseFileFetcher()
			if err != nil {
				return err
			}
		}

		// Decrypt the key
		if err := signer.DecryptKey(passphraseFetcher); err != nil {
			return err
		}
	} else {
		// User explicitly disabled signing
		fmt.Fprintf(out, "WARNING: Skipping plugin signing. This is not recommended for plugins intended for distribution.\n")
	}

	// Now create the tarball (only after signing prerequisites are met)
	// Use plugin metadata for filename: PLUGIN_NAME-SEMVER.tgz
	metadata := pluginMeta.Metadata()
	filename := fmt.Sprintf("%s-%s.tgz", metadata.Name, metadata.Version)
	tarballPath := filepath.Join(o.destination, filename)

	tarFile, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball: %w", err)
	}
	defer tarFile.Close()

	if err := plugin.CreatePluginTarball(o.pluginPath, metadata.Name, tarFile); err != nil {
		os.Remove(tarballPath)
		return fmt.Errorf("failed to create plugin tarball: %w", err)
	}
	tarFile.Close() // Ensure file is closed before signing

	// If signing was requested, sign the tarball
	if o.sign {
		// Read the tarball data
		tarballData, err := os.ReadFile(tarballPath)
		if err != nil {
			os.Remove(tarballPath)
			return fmt.Errorf("failed to read tarball for signing: %w", err)
		}

		// Sign the plugin tarball data
		sig, err := plugin.SignPlugin(tarballData, filepath.Base(tarballPath), signer)
		if err != nil {
			os.Remove(tarballPath)
			return fmt.Errorf("failed to sign plugin: %w", err)
		}

		// Write the signature
		provFile := tarballPath + ".prov"
		if err := os.WriteFile(provFile, []byte(sig), 0644); err != nil {
			os.Remove(tarballPath)
			return err
		}

		fmt.Fprintf(out, "Successfully signed. Signature written to: %s\n", provFile)
	}

	fmt.Fprintf(out, "Successfully packaged plugin and saved it to: %s\n", tarballPath)

	return nil
}

func (o *pluginPackageOptions) promptUser(name string) ([]byte, error) {
	fmt.Printf("Password for key %q >  ", name)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return pw, err
}

func (o *pluginPackageOptions) passphraseFileFetcher() (provenance.PassphraseFetcher, error) {
	file, err := openPassphraseFile(o.passphraseFile, os.Stdin)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the entire passphrase
	passphrase, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Trim any trailing newline characters (both \n and \r\n)
	passphrase = bytes.TrimRight(passphrase, "\r\n")

	return func(_ string) ([]byte, error) {
		return passphrase, nil
	}, nil
}

// copied from action.openPassphraseFile
// TODO: should we move this to pkg/action so we can reuse the func from there?
func openPassphraseFile(passphraseFile string, stdin *os.File) (*os.File, error) {
	if passphraseFile == "-" {
		stat, err := stdin.Stat()
		if err != nil {
			return nil, err
		}
		if (stat.Mode() & os.ModeNamedPipe) == 0 {
			return nil, errors.New("specified reading passphrase from stdin, without input on stdin")
		}
		return stdin, nil
	}
	return os.Open(passphraseFile)
}
