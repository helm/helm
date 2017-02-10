/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Masterminds/semver"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"k8s.io/helm/cmd/helm/helmpath"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/provenance"
	"k8s.io/helm/pkg/repo"
)

const packageDesc = `
This command packages a chart into a versioned chart archive file. If a path
is given, this will look at that path for a chart (which must contain a
Chart.yaml file) and then package that directory.

If no path is given, this will look in the present working directory for a
Chart.yaml file, and (if found) build the current directory into a chart.

Versioned chart archives are used by Helm package repositories.
`

type packageCmd struct {
	save    bool
	sign    bool
	path    string
	key     string
	keyring string
	version string
	out     io.Writer
	home    helmpath.Home
}

func newPackageCmd(out io.Writer) *cobra.Command {
	pkg := &packageCmd{
		out: out,
	}

	cmd := &cobra.Command{
		Use:   "package [flags] [CHART_PATH] [...]",
		Short: "package a chart directory into a chart archive",
		Long:  packageDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg.home = helmpath.Home(homePath())
			if len(args) == 0 {
				return fmt.Errorf("This command needs at least one argument, the path to the chart.")
			}
			if pkg.sign {
				if pkg.key == "" {
					return errors.New("--key is required for signing a package")
				}
				if pkg.keyring == "" {
					return errors.New("--keyring is required for signing a package")
				}
			}
			for i := 0; i < len(args); i++ {
				pkg.path = args[i]
				if err := pkg.run(cmd, args); err != nil {
					return err
				}
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&pkg.save, "save", true, "save packaged chart to local chart repository")
	f.BoolVar(&pkg.sign, "sign", false, "use a PGP private key to sign this package")
	f.StringVar(&pkg.key, "key", "", "name of the key to use when signing. Used if --sign is true")
	f.StringVar(&pkg.keyring, "keyring", defaultKeyring(), "location of a public keyring")
	f.StringVar(&pkg.version, "version", "", "set the version on the chart to this semver version")

	return cmd
}

func (p *packageCmd) run(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(p.path)
	if err != nil {
		return err
	}

	ch, err := chartutil.LoadDir(path)
	if err != nil {
		return err
	}

	// If version is set, modify the version.
	if len(p.version) != 0 {
		if err := setVersion(ch, p.version); err != nil {
			return err
		}
		if flagDebug {
			fmt.Fprintf(p.out, "Setting version to %s", p.version)
		}
	}

	if filepath.Base(path) != ch.Metadata.Name {
		return fmt.Errorf("directory name (%s) and Chart.yaml name (%s) must match", filepath.Base(path), ch.Metadata.Name)
	}

	if reqs, err := chartutil.LoadRequirements(ch); err == nil {
		checkDependencies(ch, reqs, p.out)
	}

	// Save to the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	name, err := chartutil.Save(ch, cwd)
	if err == nil && flagDebug {
		fmt.Fprintf(p.out, "Saved %s to current directory\n", name)
	}

	// Save to $HELM_HOME/local directory. This is second, because we don't want
	// the case where we saved here, but didn't save to the default destination.
	if p.save {
		lr := p.home.LocalRepository()
		if err := repo.AddChartToLocalRepo(ch, lr); err != nil {
			return err
		} else if flagDebug {
			fmt.Fprintf(p.out, "Saved %s to %s\n", name, lr)
		}
	}

	if p.sign {
		err = p.clearsign(name)
	}

	return err
}

func setVersion(ch *chart.Chart, ver string) error {
	// Verify that version is a SemVer, and error out if it is not.
	if _, err := semver.NewVersion(ver); err != nil {
		return err
	}

	// Set the version field on the chart.
	ch.Metadata.Version = ver
	return nil
}

func (p *packageCmd) clearsign(filename string) error {
	// Load keyring
	signer, err := provenance.NewFromKeyring(p.keyring, p.key)
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

	if flagDebug {
		fmt.Fprintln(p.out, sig)
	}

	return ioutil.WriteFile(filename+".prov", []byte(sig), 0755)
}

// promptUser implements provenance.PassphraseFetcher
func promptUser(name string) ([]byte, error) {
	fmt.Printf("Password for key %q >  ", name)
	pw, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return pw, err
}
