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
	"errors"
	"fmt"
)

// GnuPG 2.1+ stores public keys in a "keybox" (pubring.kbx) — a container
// format that interleaves OpenPGP keyblocks with GnuPG-specific metadata. It
// is not an OpenPGP packet stream, so it cannot be handed to
// openpgp.ReadKeyRing directly. A keybox is a sequence of blobs, each starting
// with:
//
//	byte  0..3   u32 blob length, big endian, including this header
//	byte  4      u8  blob type (0 empty, 1 header, 2 OpenPGP, 3 X.509)
//	byte  5      u8  blob version
//
// The first blob is a header carrying the "KBXf" magic at offset 8. OpenPGP
// blobs (type 2) record where the raw keyblock lives inside the blob:
//
//	byte  8..11  u32 keyblock offset, relative to the blob start
//	byte 12..15  u32 keyblock length
//
// Reference: kbx/keybox-blob.c in the GnuPG source tree.
const (
	kbxBlobTypeHeader  = 1
	kbxBlobTypeOpenPGP = 2

	// kbxBlobFlagEphemeral marks a blob GnuPG considers not (yet) part of
	// the keyring, e.g. written during an interrupted keyserver operation
	// (KEYBOX_FLAG_BLOB_EPHEMERAL in kbx/keybox.h). GnuPG skips such blobs
	// on every normal read (kbx/keybox-search.c), and so do we.
	kbxBlobFlagEphemeral = 0x0002

	// kbxMinBlobLen covers the length and type fields present in every blob.
	kbxMinBlobLen = 5

	// kbxOpenPGPHeaderLen is how much of an OpenPGP blob header must be
	// present for the flags, keyblock offset and keyblock length fields to
	// be readable.
	kbxOpenPGPHeaderLen = 16
)

// isKeybox reports whether data looks like a GnuPG keybox (pubring.kbx)
// image, identified by the "KBXf" magic in the mandatory first header blob.
func isKeybox(data []byte) bool {
	return len(data) >= 12 && data[4] == kbxBlobTypeHeader && string(data[8:12]) == "KBXf"
}

// isArmored reports whether data looks like an ASCII-armored keyring, as
// produced by `gpg --export --armor`.
func isArmored(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("-----BEGIN PGP"))
}

// keyboxPublicKeys extracts the OpenPGP keyblocks embedded in a keybox image
// and returns them concatenated, ready for openpgp.ReadKeyRing. Blobs of any
// other type (header, X.509, empty) are skipped, as are blobs flagged
// ephemeral, which GnuPG itself ignores when reading the keyring. Malformed
// input yields an error, never a panic.
func keyboxPublicKeys(data []byte) ([]byte, error) {
	var keyblocks bytes.Buffer
	for offset := 0; offset < len(data); {
		rest := data[offset:]
		if len(rest) < kbxMinBlobLen {
			return nil, fmt.Errorf("truncated blob header at offset %d", offset)
		}
		blobLen := binary.BigEndian.Uint32(rest)
		if blobLen < kbxMinBlobLen {
			return nil, fmt.Errorf("invalid blob length %d at offset %d", blobLen, offset)
		}
		if uint64(blobLen) > uint64(len(rest)) {
			return nil, fmt.Errorf("blob at offset %d has length %d exceeding the %d remaining bytes", offset, blobLen, len(rest))
		}
		blob := rest[:blobLen]
		if blob[4] == kbxBlobTypeOpenPGP {
			if len(blob) < kbxOpenPGPHeaderLen {
				return nil, fmt.Errorf("OpenPGP blob at offset %d is too short", offset)
			}
			flags := binary.BigEndian.Uint16(blob[6:])
			keyblockOffset := binary.BigEndian.Uint32(blob[8:])
			keyblockLen := binary.BigEndian.Uint32(blob[12:])
			if uint64(keyblockOffset)+uint64(keyblockLen) > uint64(len(blob)) {
				return nil, fmt.Errorf("OpenPGP blob at offset %d has an out-of-range keyblock", offset)
			}
			if flags&kbxBlobFlagEphemeral == 0 {
				keyblocks.Write(blob[keyblockOffset : keyblockOffset+keyblockLen])
			}
		}
		offset += int(blobLen)
	}
	if keyblocks.Len() == 0 {
		return nil, errors.New("keybox contains no OpenPGP keys")
	}
	return keyblocks.Bytes(), nil
}
