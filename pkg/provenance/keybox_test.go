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

package provenance

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// testKeybox is a GnuPG keybox (pubring.kbx) containing the helm-test
	// public key. Regenerate with testdata/regen-keyring-formats.sh.
	testKeybox = "testdata/helm-test-key.kbx"

	// testMixedKeybox is a keybox containing the RSA and Ed25519 test keys.
	testMixedKeybox = "testdata/helm-mixed-keyring.kbx"

	// testArmoredPubfile is the ASCII-armored export of the helm-test key.
	testArmoredPubfile = "testdata/helm-test-key.asc"

	// testMultiBlockArmored is two concatenated single-key armored exports
	// (cat key1.asc key2.asc), covering the RSA and Ed25519 test keys.
	testMultiBlockArmored = "testdata/helm-mixed-keyring.asc"
)

func TestIsKeybox(t *testing.T) {
	tests := []struct {
		name string
		file string
		want bool
	}{
		{"keybox", testKeybox, true},
		{"mixed keybox", testMixedKeybox, true},
		{"legacy binary keyring", testPubfile, false},
		{"armored keyring", testArmoredPubfile, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			require.NoError(t, err)
			assert.Equal(t, tt.want, isKeybox(data))
		})
	}

	t.Run("degenerate inputs", func(t *testing.T) {
		assert.False(t, isKeybox(nil))
		assert.False(t, isKeybox([]byte{}))
		assert.False(t, isKeybox([]byte("KBXf")))
		assert.False(t, isKeybox([]byte("garbage that is longer than twelve bytes")))
	})
}

func TestIsArmored(t *testing.T) {
	tests := []struct {
		name string
		file string
		want bool
	}{
		{"armored keyring", testArmoredPubfile, true},
		{"legacy binary keyring", testPubfile, false},
		{"keybox", testKeybox, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.file)
			require.NoError(t, err)
			assert.Equal(t, tt.want, isArmored(data))
		})
	}

	t.Run("degenerate inputs", func(t *testing.T) {
		assert.False(t, isArmored(nil))
		assert.False(t, isArmored([]byte("not a key")))
		assert.True(t, isArmored([]byte("\n\t -----BEGIN PGP PUBLIC KEY BLOCK-----")))
	})
}

func TestKeyboxPublicKeys(t *testing.T) {
	data, err := os.ReadFile(testKeybox)
	require.NoError(t, err)

	keys, err := keyboxPublicKeys(data)
	require.NoError(t, err)

	ring, err := openpgp.ReadKeyRing(bytes.NewReader(keys))
	require.NoError(t, err)

	require.Len(t, ring, 1)
	_, ok := ring[0].Identities[testKeyName]
	assert.True(t, ok, "expected keybox to contain %q", testKeyName)
}

func TestKeyboxPublicKeysEphemeral(t *testing.T) {
	// GnuPG flags in-progress key material as ephemeral (bit 0x0002 of the
	// blob flags at blob offset 6) and hides it from every normal read; the
	// parser must do the same.
	setFlags := func(t *testing.T, data []byte, blobStart int, flags uint16) []byte {
		t.Helper()
		require.Equal(t, byte(kbxBlobTypeOpenPGP), data[blobStart+4])
		mutated := bytes.Clone(data)
		binary.BigEndian.PutUint16(mutated[blobStart+6:], flags)
		return mutated
	}

	t.Run("all blobs ephemeral means no keys", func(t *testing.T) {
		valid, err := os.ReadFile(testKeybox)
		require.NoError(t, err)

		_, err = keyboxPublicKeys(setFlags(t, valid, 32, kbxBlobFlagEphemeral))
		assert.ErrorContains(t, err, "no OpenPGP keys")
	})

	t.Run("ephemeral blob is skipped, others kept", func(t *testing.T) {
		valid, err := os.ReadFile(testMixedKeybox)
		require.NoError(t, err)

		// Flag only the first OpenPGP blob (the RSA helm-test key).
		keys, err := keyboxPublicKeys(setFlags(t, valid, 32, kbxBlobFlagEphemeral))
		require.NoError(t, err)

		ring, err := openpgp.ReadKeyRing(bytes.NewReader(keys))
		require.NoError(t, err)

		require.Len(t, ring, 1)
		_, ok := ring[0].Identities[testKeyName]
		assert.False(t, ok, "expected the ephemeral-flagged %q blob to be skipped", testKeyName)
	})
}

func TestKeyboxPublicKeysMalformed(t *testing.T) {
	valid, err := os.ReadFile(testKeybox)
	require.NoError(t, err)

	// The mutations below rely on the fixture layout: a 32-byte header blob
	// followed by an OpenPGP blob.
	const blobStart = 32
	require.Greater(t, len(valid), blobStart+16)
	require.Equal(t, byte(kbxBlobTypeOpenPGP), valid[blobStart+4])

	mutate := func(offset int, value uint32) []byte {
		data := bytes.Clone(valid)
		binary.BigEndian.PutUint32(data[offset:], value)
		return data
	}

	tests := []struct {
		name string
		data []byte
	}{
		{"header only, no keys", valid[:blobStart]},
		{"truncated inside blob header", valid[:blobStart+2]},
		{"truncated inside blob body", valid[:blobStart+16]},
		{"zero blob length", mutate(blobStart, 0)},
		{"blob length below minimum", mutate(blobStart, 4)},
		{"blob length past end of data", mutate(blobStart, uint32(len(valid))+1)},
		{"keyblock offset out of range", mutate(blobStart+8, uint32(len(valid)))},
		{"keyblock length out of range", mutate(blobStart+12, uint32(len(valid)))},
		{"keyblock offset overflow", mutate(blobStart+8, ^uint32(0))},
		{"keyblock length overflow", mutate(blobStart+12, ^uint32(0))},
		{"openpgp blob shorter than its header", append(bytes.Clone(valid[:blobStart]), 0, 0, 0, 8, kbxBlobTypeOpenPGP, 1, 0, 0)},
		{"empty input", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := keyboxPublicKeys(tt.data)
			assert.Error(t, err)
		})
	}
}
