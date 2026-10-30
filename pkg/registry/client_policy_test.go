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

package registry

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/oras-project/oras-go/v3/registry/remote/policy"
	"github.com/stretchr/testify/require"
)

// TestPush_PolicyScopes verifies that a push honours reference-level policy
// (reject) but not signature requirements: a chart cannot be signed before
// its manifest exists, so signedBy only applies when the chart is read.
func TestPush_PolicyScopes(t *testing.T) {
	host := startInMemoryRegistry(t)

	policyPath := filepath.Join(t.TempDir(), "policy.json")
	require.NoError(t, os.WriteFile(policyPath, fmt.Appendf(nil, `{
  "default": [{"type": "insecureAcceptAnything"}],
  "transports": {"docker": {
    %[1]q: [{"type": "signedBy", "keyType": "GPGKeys", "keyData": "unused"}],
    %[2]q: [{"type": "reject"}]
  }}
}`, host+"/signed", host+"/blocked"), 0o600))

	c, err := NewClient(
		ClientOptWriter(io.Discard),
		ClientOptCredentialsFile(filepath.Join(t.TempDir(), "config.json")),
		ClientOptPlainHTTP(),
		ClientOptContainersConfig(true),
		ClientOptPolicyEnforcement(true),
		withPolicyConfigPath(policyPath),
	)
	require.NoError(t, err)

	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	require.NoError(t, err)
	meta, err := extractChartMeta(chartData)
	require.NoError(t, err)

	signedRef := fmt.Sprintf("%s/signed/%s:%s", host, meta.Name, meta.Version)
	_, err = c.Push(chartData, signedRef)
	require.NoError(t, err, "push to a signedBy scope must not require a signature")

	_, err = c.Pull(signedRef)
	require.Error(t, err, "pull from a signedBy scope must still verify the signature")

	_, err = c.Push(chartData, fmt.Sprintf("%s/blocked/%s:%s", host, meta.Name, meta.Version))
	require.ErrorIs(t, err, policy.ErrDenied, "push to a rejected scope must be denied")
}
