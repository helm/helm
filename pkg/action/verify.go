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
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/pkg/provenance"
)

// Verify is the action for building a given chart's Verify tree.
//
// It provides the implementation of 'helm verify'.
type Verify struct {
	Keyring string
}

// NewVerify creates a new Verify object with the given configuration.
func NewVerify() *Verify {
	return &Verify{}
}

// Run executes 'helm verify'.
func (v *Verify) Run(chartfile string) error {
	_, err := VerifyChart(chartfile, v.Keyring)
	return err
}

// VerifyChart takes a path to a chart archive and a keyring, and verifies the chart.
//
// It assumes that a chart archive file is accompanied by a provenance file whose
// name is the archive file name plus the ".prov" extension.
func VerifyChart(path, keyring string) (*provenance.Verification, error) {
	// For now, error out if it's not a tar file.
	if fi, err := os.Stat(path); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("unpacked charts cannot be verified")
	} else if strings.ToLower(filepath.Ext(path)) != ".tgz" {
		return nil, errors.New("chart must be a tgz file")
	}

	provfile := path + ".prov"
	if _, err := os.Stat(provfile); err != nil {
		return nil, errors.Wrapf(err, "could not load provenance file %s", provfile)
	}

	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to load keyring")
	}
	return sig.Verify(path, provfile)
}
