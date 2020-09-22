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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
)

const packageDesc = `
This command packages a chart into a versioned chart archive file. If a path
is given, this will look at that path for a chart (which must contain a
Chart.yaml file) and then package that directory.

Versioned chart archives are used by Helm package repositories.

To sign a chart, use the '--sign' flag. In most cases, you should also
provide '--keyring path/to/secret/keys' and '--key keyname'.

  $ helm package --sign ./mychart --key mykey --keyring ~/.gnupg/secring.gpg

If '--keyring' is not specified, Helm usually defaults to the public keyring
unless your environment is otherwise configured.
`

func newPackageCmd(out io.Writer) *cobra.Command {
	client := action.NewPackage()
	valueOpts := &values.Options{}

	cmd := &cobra.Command{
		Use:   "package [CHART_PATH] [...]",
		Short: "package a chart directory into a chart archive",
		Long:  packageDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.Errorf("need at least one argument, the path to the chart")
			}
			if client.Sign {
				if client.Key == "" {
					return errors.New("--key is required for signing a package")
				}
				if client.Keyring == "" {
					return errors.New("--keyring is required for signing a package")
				}
			}
			client.RepositoryConfig = settings.RepositoryConfig
			client.RepositoryCache = settings.RepositoryCache
			p := getter.All(settings)
			vals, err := valueOpts.MergeValues(p)
			if err != nil {
				return err
			}

			for i := 0; i < len(args); i++ {
				path, err := filepath.Abs(args[i])
				if err != nil {
					return err
				}
				if _, err := os.Stat(args[i]); err != nil {
					return err
				}

				if client.DependencyUpdate {
					downloadManager := &downloader.Manager{
						Out:              ioutil.Discard,
						ChartPath:        path,
						Keyring:          client.Keyring,
						Getters:          p,
						Debug:            settings.Debug,
						RepositoryConfig: settings.RepositoryConfig,
						RepositoryCache:  settings.RepositoryCache,
					}

					if err := downloadManager.Update(); err != nil {
						return err
					}
				}
				p, err := client.Run(path, vals)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "Successfully packaged chart and saved it to: %s\n", p)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&client.Sign, "sign", false, "use a PGP private key to sign this package")
	f.StringVar(&client.Key, "key", "", "name of the key to use when signing. Used if --sign is true")
	f.StringVar(&client.Keyring, "keyring", defaultKeyring(), "location of a public keyring")
	f.StringVar(&client.PassphraseFile, "passphrase-file", "", `location of a file which contains the passphrase for the signing key. Use "-" in order to read from stdin.`)
	f.StringVar(&client.Version, "version", "", "set the version on the chart to this semver version")
	f.StringVar(&client.AppVersion, "app-version", "", "set the appVersion on the chart to this version")
	f.StringVarP(&client.Destination, "destination", "d", ".", "location to write the chart.")
	f.BoolVarP(&client.DependencyUpdate, "dependency-update", "u", false, `update dependencies from "Chart.yaml" to dir "charts/" before packaging`)

	return cmd
}
