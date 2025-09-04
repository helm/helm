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

package plugin

import (
	"path/filepath"

	"helm.sh/helm/v4/pkg/provenance"
)

// VerifyPlugin verifies plugin data against a signature using data in memory.
func VerifyPlugin(archiveData, provData []byte, filename, keyring string) (*provenance.Verification, error) {
	// Create signatory from keyring
	sig, err := provenance.NewFromKeyring(keyring, "")
	if err != nil {
		return nil, err
	}

	// Use the new VerifyData method directly
	return sig.Verify(archiveData, provData, filename)
}

// isTarball checks if a file has a tarball extension
func IsTarball(filename string) bool {
	return filepath.Ext(filename) == ".gz" || filepath.Ext(filename) == ".tgz"
}
