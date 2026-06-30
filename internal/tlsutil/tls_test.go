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

package tlsutil

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tlsTestDir = "../../testdata"

const (
	testCaCertFile = "rootca.crt"
	testCertFile   = "crt.pem"
	testKeyFile    = "key.pem"
)

func testfile(t *testing.T, file string) (path string) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join(tlsTestDir, file))
	require.NoError(t, err, "error getting absolute path to test file %q", file)
	return path
}

func TestNewTLSConfig(t *testing.T) {
	certFile := testfile(t, testCertFile)
	keyFile := testfile(t, testKeyFile)
	caCertFile := testfile(t, testCaCertFile)
	insecureSkipTLSVerify := false

	{
		cfg, err := NewTLSConfig(
			WithInsecureSkipVerify(insecureSkipTLSVerify),
			WithCertKeyPairFiles(certFile, keyFile),
			WithCAFile(caCertFile),
		)
		assert.NoError(t, err)

		require.Len(t, cfg.Certificates, 1)
		require.False(t, cfg.InsecureSkipVerify, "insecure skip verify mismatch, expecting false")
		require.NotNil(t, cfg.RootCAs, "mismatch tls RootCAs, expecting non-nil")
	}
	{
		cfg, err := NewTLSConfig(
			WithInsecureSkipVerify(insecureSkipTLSVerify),
			WithCAFile(caCertFile),
		)
		assert.NoError(t, err)

		require.Empty(t, cfg.Certificates)
		require.False(t, cfg.InsecureSkipVerify, "insecure skip verify mismatch, expecting false")
		require.NotNil(t, cfg.RootCAs, "mismatch tls RootCAs, expecting non-nil")
	}

	{
		cfg, err := NewTLSConfig(
			WithInsecureSkipVerify(insecureSkipTLSVerify),
			WithCertKeyPairFiles(certFile, keyFile),
		)
		assert.NoError(t, err)

		require.Len(t, cfg.Certificates, 1)
		require.False(t, cfg.InsecureSkipVerify, "insecure skip verify mismatch, expecting false")
		require.Nil(t, cfg.RootCAs, "mismatch tls RootCAs, expecting nil")
	}
}
