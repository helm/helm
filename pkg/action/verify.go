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
	"helm.sh/helm/v3/pkg/downloader"
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
	_, err := downloader.VerifyChart(chartfile, v.Keyring)
	return err
}
