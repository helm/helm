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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVerify(t *testing.T) {
	client := NewVerify()

	assert.NotNil(t, client)
}

func TestVerifyRun(t *testing.T) {
	client := NewVerify()

	client.Keyring = "../downloader/testdata/helm-test-key.pub"
	output, err := client.Run("../downloader/testdata/signtest-0.1.0.tgz")
	assert.Contains(t, output, "Signed by:")
	assert.Contains(t, output, "Using Key With Fingerprint:")
	assert.Contains(t, output, "Chart Hash Verified:")
	require.NoError(t, err)
}

func TestVerifyRun_DownloadError(t *testing.T) {
	client := NewVerify()
	output, err := client.Run("invalid-chart-path")
	require.Error(t, err)
	assert.Empty(t, output)
}
