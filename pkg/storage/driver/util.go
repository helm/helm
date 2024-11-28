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

package driver // import "helm.sh/helm/v3/pkg/storage/driver"

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	rspb "helm.sh/helm/v3/pkg/release"
)

var b64 = base64.StdEncoding

var magicGzip = []byte{0x1f, 0x8b, 0x08}

var systemLabels = []string{"name", "owner", "status", "version", "createdAt", "modifiedAt"}

// encodeRelease encodes a release returning a base64 encoded
// gzipped string representation, or error.
func encodeRelease(rls *rspb.Release) (string, error) {
	b, err := json.Marshal(rls)
	if err != nil {
		return "", err
	}

	encryptionAlgorithm := os.Getenv("HELM_RELEASE_ENCRYPTION_ALGO")
	encryptionKey := os.Getenv("HELM_RELEASE_ENCRYPTION_KEY")

	var data []byte

	switch encryptionAlgorithm {
	case "aes":
		data, err = encryptDataAES(b, []byte(encryptionKey))
	case "":
		data = b
	default:
		return "", fmt.Errorf("error encrypting release: unknown algorithm %v", encryptionAlgorithm)
	}

	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err = w.Write(data); err != nil {
		return "", err
	}
	w.Close()

	return b64.EncodeToString(buf.Bytes()), nil
}

// decodeRelease decodes the bytes of data into a release
// type. Data must contain a base64 encoded gzipped string of a
// valid release, otherwise an error is returned.
func decodeRelease(data string) (*rspb.Release, error) {
	// base64 decode string
	b, err := b64.DecodeString(data)
	if err != nil {
		return nil, err
	}

	// For backwards compatibility with releases that were stored before
	// compression was introduced we skip decompression if the
	// gzip magic header is not found
	if len(b) > 3 && bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	encryptionAlgorithm := os.Getenv("HELM_RELEASE_ENCRYPTION_ALGO")
	encryptionKey := os.Getenv("HELM_RELEASE_ENCRYPTION_KEY")

	var releaseBytes []byte

	switch encryptionAlgorithm {
	case "aes":
		releaseBytes, err = decryptDataAES(b, []byte(encryptionKey))
	case "":
		releaseBytes = b
	default:
		return nil, fmt.Errorf("error decrypting release: unknown algorithm %v", encryptionAlgorithm)
	}

	if err != nil {
		return nil, err
	}

	var rls rspb.Release
	// unmarshal release object bytes
	if err := json.Unmarshal(releaseBytes, &rls); err != nil {
		return nil, err
	}
	return &rls, nil
}

// encryptDataAES encrypts data using AES in Galois/Counter Mode (GCM) with the
// provided key. The result includes both nonce and encrypted content.
func encryptDataAES(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)
	return append(nonce, ciphertext...), nil
}

// decryptDataAES decrypts data using AES (Advanced Encryption Standard) in
// Galois/Counter Mode (GCM). It takes encrypted data and an encryption key,
// returning the original plaintext content.
func decryptDataAES(encryptedData []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("error decrypting data: ciphertext too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]

	decryptedData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return decryptedData, nil
}

// Checks if label is system
func isSystemLabel(key string) bool {
	for _, v := range GetSystemLabels() {
		if key == v {
			return true
		}
	}
	return false
}

// Removes system labels from labels map
func filterSystemLabels(lbs map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range lbs {
		if !isSystemLabel(k) {
			result[k] = v
		}
	}
	return result
}

// Checks if labels array contains system labels
func ContainsSystemLabels(lbs map[string]string) bool {
	for k := range lbs {
		if isSystemLabel(k) {
			return true
		}
	}
	return false
}

func GetSystemLabels() []string {
	return systemLabels
}
