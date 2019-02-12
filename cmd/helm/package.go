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
	"strings"
	"syscall"

	"github.com/Masterminds/semver"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/chart/loader"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/provenance"
)

const packageDesc = `
This command packages a chart into a versioned chart archive file. If a path
is given, this will look at that path for a chart (which must contain a
Chart.yaml file) and then package that directory.

If no path is given, this will look in the present working directory for a
Chart.yaml file, and (if found) build the current directory into a chart.

Versioned chart archives are used by Helm package repositories.
`

type packageOptions struct {
	appVersion       string // --app-version
	dependencyUpdate bool   // --dependency-update
	destination      string // --destination
	key              string // --key
	keyring          string // --keyring
	sign             bool   // --sign
	version          string // --version

	valuesOptions

	path string

	home helmpath.Home
}

func newPackageCmd(out io.Writer) *cobra.Command {
	o := &packageOptions{}

	cmd := &cobra.Command{
		Use:   "package [CHART_PATH] [...]",
		Short: "package a chart directory into a chart archive",
		Long:  packageDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			o.home = settings.Home
			if len(args) == 0 {
				return errors.Errorf("need at least one argument, the path to the chart")
			}
			if o.sign {
				if o.key == "" {
					return errors.New("--key is required for signing a package")
				}
				if o.keyring == "" {
					return errors.New("--keyring is required for signing a package")
				}
			}
			for i := 0; i < len(args); i++ {
				o.path = args[i]
				if err := o.run(out); err != nil {
					return err
				}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&o.sign, "sign", false, "use a PGP private key to sign this package")
	f.StringVar(&o.key, "key", "", "name of the key to use when signing. Used if --sign is true")
	f.StringVar(&o.keyring, "keyring", defaultKeyring(), "location of a public keyring")
	f.StringVar(&o.version, "version", "", "set the version on the chart to this semver version")
	f.StringVar(&o.appVersion, "app-version", "", "set the appVersion on the chart to this version")
	f.StringVarP(&o.destination, "destination", "d", ".", "location to write the chart.")
	f.BoolVarP(&o.dependencyUpdate, "dependency-update", "u", false, `update dependencies from "Chart.yaml" to dir "charts/" before packaging`)
	o.valuesOptions.addFlags(f)

	return cmd
}

func (o *packageOptions) run(out io.Writer) error {
	path, err := filepath.Abs(o.path)
	if err != nil {
		return err
	}

	if o.dependencyUpdate {
		downloadManager := &downloader.Manager{
			Out:       out,
			ChartPath: path,
			HelmHome:  settings.Home,
			Keyring:   o.keyring,
			Getters:   getter.All(settings),
			Debug:     settings.Debug,
		}

		if err := downloadManager.Update(); err != nil {
			return err
		}
	}

	ch, err := loader.LoadDir(path)
	if err != nil {
		return err
	}

	chartType := ch.Metadata.Type
	if chartType != "" && !strings.EqualFold(chartType, "library") &&
		!strings.EqualFold(chartType, "application") {
		return errors.New("Invalid chart type. Valid types are: application or library")
	}

	overrideVals, err := o.mergedValues()
	if err != nil {
		return err
	}
	combinedVals, err := chartutil.CoalesceValues(ch, overrideVals)
	if err != nil {
		return err
	}
	ch.Values = combinedVals

	// If version is set, modify the version.
	if len(o.version) != 0 {
		if err := setVersion(ch, o.version); err != nil {
			return err
		}
		debug("Setting version to %s", o.version)
	}

	if o.appVersion != "" {
		ch.Metadata.AppVersion = o.appVersion
		debug("Setting appVersion to %s", o.appVersion)
	}

	if reqs := ch.Metadata.Dependencies; reqs != nil {
		if err := checkDependencies(ch, reqs); err != nil {
			return err
		}
	}

	var dest string
	if o.destination == "." {
		// Save to the current working directory.
		dest, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		// Otherwise save to set destination
		dest = o.destination
	}

	name, err := chartutil.Save(ch, dest)
	if err != nil {
		return errors.Wrap(err, "failed to save")
	}
	fmt.Fprintf(out, "Successfully packaged chart and saved it to: %s\n", name)

	if o.sign {
		err = o.clearsign(name)
	}

	return err
}

func setVersion(ch *chart.Chart, ver string) error {
	// Verify that version is a Version, and error out if it is not.
	if _, err := semver.NewVersion(ver); err != nil {
		return err
	}

	// Set the version field on the chart.
	ch.Metadata.Version = ver
	return nil
}

func (o *packageOptions) clearsign(filename string) error {
	// Load keyring
	signer, err := provenance.NewFromKeyring(o.keyring, o.key)
	if err != nil {
		return err
	}

	if err := signer.DecryptKey(promptUser); err != nil {
		return err
	}

	sig, err := signer.ClearSign(filename)
	if err != nil {
		return err
	}

	debug(sig)

	return ioutil.WriteFile(filename+".prov", []byte(sig), 0755)
}

// promptUser implements provenance.PassphraseFetcher
func promptUser(name string) ([]byte, error) {
	fmt.Printf("Password for key %q >  ", name)
	pw, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return pw, err
}
