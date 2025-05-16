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

package repotest

import (
	"crypto/tls"
	"path/filepath"
	"testing"

	"helm.sh/helm/v4/internal/tlsutil"

	"github.com/stretchr/testify/require"
)

func MakeTestTLSConfig(t *testing.T, path string) *tls.Config {
	t.Helper()
	ca, pub, priv := filepath.Join(path, "rootca.crt"), filepath.Join(path, "crt.pem"), filepath.Join(path, "key.pem")

	insecure := false
	tlsConf, err := tlsutil.NewTLSConfig(
		tlsutil.WithInsecureSkipVerify(insecure),
		tlsutil.WithCertKeyPairFiles(pub, priv),
		tlsutil.WithCAFile(ca),
	)
	//require.Nil(t, err, err.Error())
	require.Nil(t, err)

	tlsConf.ServerName = "helm.sh"

	return tlsConf
}
