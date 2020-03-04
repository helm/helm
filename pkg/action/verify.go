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
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/downloader"
)

// Verify is the action for building a given chart's Verify tree.
//
// It provides the implementation of 'helm verify'.
type Verify struct {
	Keyring string
	Out     string
}

// NewVerify creates a new Verify object with the given configuration.
func NewVerify() *Verify {
	return &Verify{}
}

// Run executes 'helm verify'.
func (v *Verify) Run(chartfile string) error {
	var out strings.Builder
	p, err := downloader.VerifyChart(chartfile, v.Keyring)
	if err != nil {
		return err
	}

	for name := range p.SignedBy.Identities {
		fmt.Fprintf(&out, "Signed by: %v\n", name)
	}
	fmt.Fprintf(&out, "Using Key With Fingerprint: %X\n", p.SignedBy.PrimaryKey.Fingerprint)
	fmt.Fprintf(&out, "Chart Hash Verified: %s\n", p.FileHash)

	// TODO(mattfarina): The output is set as a property rather than returned
	// to maintain the Go API. In Helm v4 this function should return the out
	// and the property on the struct can be removed.
	v.Out = out.String()

	return nil
}
