/*
Copyright 2016 The Kubernetes Authors All rights reserved.
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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pgperrors "golang.org/x/crypto/openpgp/errors"
)

const (
	// testKeyFile is the secret key.
	// Generating keys should be done with `gpg --gen-key`. The current key
	// was generated to match Go's defaults (RSA/RSA 2048). It has no pass
	// phrase. Use `gpg --export-secret-keys helm-test` to export the secret.
	testKeyfile = "testdata/helm-test-key.secret"

	// testPubfile is the public key file.
	// Use `gpg --export helm-test` to export the public key.
	testPubfile = "testdata/helm-test-key.pub"

	// Generated name for the PGP key in testKeyFile.
	testKeyName = `Helm Testing (This key should only be used for testing. DO NOT TRUST.) <helm-testing@helm.sh>`

	testChartfile = "testdata/hashtest-1.2.3.tgz"

	// testSigBlock points to a signature generated by an external tool.
	// This file was generated with GnuPG:
	// gpg --clearsign -u helm-test --openpgp testdata/msgblock.yaml
	testSigBlock = "testdata/msgblock.yaml.asc"

	// testTamperedSigBlock is a tampered copy of msgblock.yaml.asc
	testTamperedSigBlock = "testdata/msgblock.yaml.tampered"

	// testSumfile points to a SHA256 sum generated by an external tool.
	// We always want to validate against an external tool's representation to
	// verify that we haven't done something stupid. This file was generated
	// with shasum.
	// shasum -a 256 hashtest-1.2.3.tgz > testdata/hashtest.sha256
	testSumfile = "testdata/hashtest.sha256"
)

// testMessageBlock represents the expected message block for the testdata/hashtest chart.
const testMessageBlock = `description: Test chart versioning
name: hashtest
version: 1.2.3

...
files:
  hashtest-1.2.3.tgz: sha256:8e90e879e2a04b1900570e1c198755e46e4706d70b0e79f5edabfac7900e4e75
`

func TestMessageBlock(t *testing.T) {
	out, err := messageBlock(testChartfile)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()

	if got != testMessageBlock {
		t.Errorf("Expected:\n%q\nGot\n%q\n", testMessageBlock, got)
	}
}

func TestParseMessageBlock(t *testing.T) {
	md, sc, err := parseMessageBlock([]byte(testMessageBlock))
	if err != nil {
		t.Fatal(err)
	}

	if md.Name != "hashtest" {
		t.Errorf("Expected name %q, got %q", "hashtest", md.Name)
	}

	if lsc := len(sc.Files); lsc != 1 {
		t.Errorf("Expected 1 file, got %d", lsc)
	}

	if hash, ok := sc.Files["hashtest-1.2.3.tgz"]; !ok {
		t.Errorf("hashtest file not found in Files")
	} else if hash != "sha256:8e90e879e2a04b1900570e1c198755e46e4706d70b0e79f5edabfac7900e4e75" {
		t.Errorf("Unexpected hash: %q", hash)
	}
}

func TestLoadKey(t *testing.T) {
	k, err := loadKey(testKeyfile)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := k.Identities[testKeyName]; !ok {
		t.Errorf("Expected to load a key for user %q", testKeyName)
	}
}

func TestLoadKeyRing(t *testing.T) {
	k, err := loadKeyRing(testPubfile)
	if err != nil {
		t.Fatal(err)
	}

	if len(k) > 1 {
		t.Errorf("Expected 1, got %d", len(k))
	}

	for _, e := range k {
		if ii, ok := e.Identities[testKeyName]; !ok {
			t.Errorf("Expected %s in %v", testKeyName, ii)
		}
	}
}

func TestDigest(t *testing.T) {
	f, err := os.Open(testChartfile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	hash, err := Digest(f)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := readSumFile(testSumfile)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(sig, hash) {
		t.Errorf("Expected %s to be in %s", hash, sig)
	}
}

func TestNewFromFiles(t *testing.T) {
	s, err := NewFromFiles(testKeyfile, testPubfile)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := s.Entity.Identities[testKeyName]; !ok {
		t.Errorf("Expected to load a key for user %q", testKeyName)
	}
}

func TestDigestFile(t *testing.T) {
	hash, err := DigestFile(testChartfile)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := readSumFile(testSumfile)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(sig, hash) {
		t.Errorf("Expected %s to be in %s", hash, sig)
	}
}

func TestClearSign(t *testing.T) {
	signer, err := NewFromFiles(testKeyfile, testPubfile)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := signer.ClearSign(testChartfile)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Sig:\n%s", sig)

	if !strings.Contains(sig, testMessageBlock) {
		t.Errorf("expected message block to be in sig: %s", sig)
	}
}

func TestDecodeSignature(t *testing.T) {
	// Unlike other tests, this does a round-trip test, ensuring that a signature
	// generated by the library can also be verified by the library.

	signer, err := NewFromFiles(testKeyfile, testPubfile)
	if err != nil {
		t.Fatal(err)
	}

	sig, err := signer.ClearSign(testChartfile)
	if err != nil {
		t.Fatal(err)
	}

	f, err := ioutil.TempFile("", "helm-test-sig-")
	if err != nil {
		t.Fatal(err)
	}

	tname := f.Name()
	defer func() {
		os.Remove(tname)
	}()
	f.WriteString(sig)
	f.Close()

	sig2, err := signer.decodeSignature(tname)
	if err != nil {
		t.Fatal(err)
	}

	by, err := signer.verifySignature(sig2)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := by.Identities[testKeyName]; !ok {
		t.Errorf("Expected identity %q", testKeyName)
	}
}

func TestVerify(t *testing.T) {
	signer, err := NewFromFiles(testKeyfile, testPubfile)
	if err != nil {
		t.Fatal(err)
	}

	if ver, err := signer.Verify(testChartfile, testSigBlock); err != nil {
		t.Errorf("Failed to pass verify. Err: %s", err)
	} else if len(ver.FileHash) == 0 {
		t.Error("Verification is missing hash.")
	} else if ver.SignedBy == nil {
		t.Error("No SignedBy field")
	} else if ver.FileName != filepath.Base(testChartfile) {
		t.Errorf("FileName is unexpectedly %q", ver.FileName)
	}

	if _, err = signer.Verify(testChartfile, testTamperedSigBlock); err == nil {
		t.Errorf("Expected %s to fail.", testTamperedSigBlock)
	}

	switch err.(type) {
	case pgperrors.SignatureError:
		t.Logf("Tampered sig block error: %s (%T)", err, err)
	default:
		t.Errorf("Expected invalid signature error, got %q (%T)", err, err)
	}
}

// readSumFile reads a file containing a sum generated by the UNIX shasum tool.
func readSumFile(sumfile string) (string, error) {
	data, err := ioutil.ReadFile(sumfile)
	if err != nil {
		return "", err
	}

	sig := string(data)
	parts := strings.SplitN(sig, " ", 2)
	return parts[0], nil
}
