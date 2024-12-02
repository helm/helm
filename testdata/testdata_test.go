package testdata

import (
	"crypto/x509"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadTLSConfig(t *testing.T) {

	insecureSkipVerify := false

	tlsConfig, err := ReadTLSConfig(insecureSkipVerify)

	require.Nil(t, err)
	assert.Equal(t, insecureSkipVerify, tlsConfig.InsecureSkipVerify)

	require.Len(t, tlsConfig.Certificates, 1)
	require.Len(t, tlsConfig.Certificates[0].Certificate, 1)

	leaf, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
	assert.Nil(t, err)

	assert.Equal(t, []string{"helm.sh"}, leaf.DNSNames)
	assert.Equal(t, []net.IP{{127, 0, 0, 1}}, leaf.IPAddresses)
}
