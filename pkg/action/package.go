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

package action

import (
	"bufio"
	"fmt"
	"os"
	"syscall"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"golang.org/x/term"

	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/provenance"
)

// Package is the action for packaging a chart.
//
// It provides the implementation of 'helm package'.
type Package struct {
	Sign             bool
	Key              string
	Keyring          string
	PassphraseFile   string
	Version          string
	AppVersion       string
	Destination      string
	DependencyUpdate bool

	RepositoryConfig string
	RepositoryCache  string
}

// NewPackage creates a new Package object with the given configuration.
func NewPackage() *Package {
	return &Package{}
}

// Run executes 'helm package' against the given chart and returns the path to the packaged chart.
func (p *Package) Run(path string, _ map[string]interface{}) (string, error) {
	ch, err := loader.LoadDir(path)
	if err != nil {
		return "", err
	}

	// If version is set, modify the version.
	if p.Version != "" {
		ch.Metadata.Version = p.Version
	}

	if err := validateVersion(ch.Metadata.Version); err != nil {
		return "", err
	}

	if p.AppVersion != "" {
		ch.Metadata.AppVersion = p.AppVersion
	}

	if reqs := ch.Metadata.Dependencies; reqs != nil {
		if err := CheckDependencies(ch, reqs); err != nil {
			return "", err
		}
	}

	var dest string
	if p.Destination == "." {
		// Save to the current working directory.
		dest, err = os.Getwd()
		if err != nil {
			return "", err
		}
	} else {
		// Otherwise save to set destination
		dest = p.Destination
	}

	name, err := chartutil.Save(ch, dest)
	if err != nil {
		return "", errors.Wrap(err, "failed to save")
	}

	if p.Sign {
		err = p.Clearsign(name)
	}

	return name, err
}

// validateVersion Verify that version is a Version, and error out if it is not.
func validateVersion(ver string) error {
	if _, err := semver.NewVersion(ver); err != nil {
		return err
	}
	return nil
}

// Clearsign signs a chart
func (p *Package) Clearsign(filename string) error {
	// Load keyring
	signer, err := provenance.NewFromKeyring(p.Keyring, p.Key)
	if err != nil {
		return err
	}

	passphraseFetcher := promptUser
	if p.PassphraseFile != "" {
		passphraseFetcher, err = passphraseFileFetcher(p.PassphraseFile, os.Stdin)
		if err != nil {
			return err
		}
	}

	if err := signer.DecryptKey(passphraseFetcher); err != nil {
		return err
	}

	sig, err := signer.ClearSign(filename)
	if err != nil {
		return err
	}

	return os.WriteFile(filename+".prov", []byte(sig), 0644)
}

// promptUser implements provenance.PassphraseFetcher
func promptUser(name string) ([]byte, error) {
	fmt.Printf("Password for key %q >  ", name)
	// syscall.Stdin is not an int in all environments and needs to be coerced
	// into one there (e.g., Windows)
	pw, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return pw, err
}

func passphraseFileFetcher(passphraseFile string, stdin *os.File) (provenance.PassphraseFetcher, error) {
	file, err := openPassphraseFile(passphraseFile, stdin)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	passphrase, _, err := reader.ReadLine()
	if err != nil {
		return nil, err
	}
	return func(name string) ([]byte, error) {
		return passphrase, nil
	}, nil
}

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
