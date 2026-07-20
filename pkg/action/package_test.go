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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test/ensure"
)

func TestPassphraseFileFetcher(t *testing.T) {
	secret := "secret"
	directory := ensure.TempFile(t, "passphrase-file", []byte(secret))
	testPkg := NewPackage()

	fetcher, err := testPkg.passphraseFileFetcher(path.Join(directory, "passphrase-file"), nil)
	if err != nil {
		t.Fatal("Unable to create passphraseFileFetcher", err)
	}

	passphrase, err := fetcher("key")
	if err != nil {
		t.Fatal("Unable to fetch passphrase")
	}

	if string(passphrase) != secret {
		t.Errorf("Expected %s got %s", secret, string(passphrase))
	}
}

func TestPassphraseFileFetcher_WithLineBreak(t *testing.T) {
	secret := "secret"
	directory := ensure.TempFile(t, "passphrase-file", []byte(secret+"\n\n."))
	testPkg := NewPackage()

	fetcher, err := testPkg.passphraseFileFetcher(path.Join(directory, "passphrase-file"), nil)
	if err != nil {
		t.Fatal("Unable to create passphraseFileFetcher", err)
	}

	passphrase, err := fetcher("key")
	if err != nil {
		t.Fatal("Unable to fetch passphrase")
	}

	if string(passphrase) != secret {
		t.Errorf("Expected %s got %s", secret, string(passphrase))
	}
}

func TestPassphraseFileFetcher_WithInvalidStdin(t *testing.T) {
	directory := t.TempDir()
	testPkg := NewPackage()

	stdin, err := os.CreateTemp(directory, "non-existing")
	if err != nil {
		t.Fatal("Unable to create test file", err)
	}

	if _, err := testPkg.passphraseFileFetcher("-", stdin); err == nil {
		t.Error("Expected passphraseFileFetcher returning an error")
	}
}

func TestPassphraseFileFetcher_WithStdinAndMultipleFetches(t *testing.T) {
	testPkg := NewPackage()
	stdin, w, err := os.Pipe()
	if err != nil {
		t.Fatal("Unable to create pipe", err)
	}

	passphrase := "secret-from-stdin"

	go func() {
		_, err := w.WriteString(passphrase + "\n")
		assert.NoError(t, err)
	}()

	for range 4 {
		fetcher, err := testPkg.passphraseFileFetcher("-", stdin)
		if err != nil {
			t.Errorf("Expected passphraseFileFetcher to not return an error, but got %v", err)
		}

		pass, err := fetcher("key")
		if err != nil {
			t.Errorf("Expected passphraseFileFetcher invocation to succeed, failed with %v", err)
		}

		if string(pass) != string(passphrase) {
			t.Errorf("Expected multiple passphrase fetch to return %q, got %q", passphrase, pass)
		}
	}
}

func TestValidateVersion(t *testing.T) {
	type args struct {
		ver string
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			"normal semver version",
			args{
				ver: "1.1.3-23658",
			},
			nil,
		},
		{
			"Pre version number starting with 0",
			args{
				ver: "1.1.3-023658",
			},
			semver.ErrSegmentStartsZero,
		},
		{
			"Invalid version number",
			args{
				ver: "1.1.3.sd.023658",
			},
			semver.ErrInvalidSemVer,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateVersion(tt.args.ver); err != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Expected {%v}, got {%v}", tt.wantErr, err)
				}
			}
		})
	}
}

func TestRun_ErrorPath(t *testing.T) {
	client := NewPackage()
	_, err := client.Run("err-path", nil)
	require.Error(t, err)
}

func TestRun(t *testing.T) {
	chartPath := "testdata/charts/chart-with-schema"
	client := NewPackage()
	filename, err := client.Run(chartPath, nil)
	require.NoError(t, err)
	require.Equal(t, "empty-0.1.0.tgz", filename)
	require.NoError(t, os.Remove(filename))
}

func TestRunWithSourceDateEpochAndLock(t *testing.T) {
	tmp := t.TempDir()
	epoch := time.Unix(1609459200, 0).UTC()

	// Build twice with the same SourceDateEpoch and assert byte-identical output.
	var first []byte
	for i := range 2 {
		client := NewPackage()
		client.Destination = tmp
		client.SourceDateEpoch = &epoch

		dest, err := client.Run("testdata/charts/chart-with-lock", nil)
		require.NoError(t, err)

		data, err := os.ReadFile(dest)
		require.NoError(t, err)

		if i == 0 {
			first = data
		} else {
			require.Equal(t, first, data, "two builds with the same SourceDateEpoch must be byte-identical")
		}
		require.NoError(t, os.Remove(dest))
	}

	// Open the archive and inspect Chart.lock content.
	gz, err := gzip.NewReader(bytes.NewReader(first))
	require.NoError(t, err)
	defer gz.Close()

	tr := tar.NewReader(gz)
	var lockData []byte
	var lockHdr *tar.Header
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if strings.HasSuffix(hdr.Name, "Chart.lock") {
			lockHdr = hdr
			var buf bytes.Buffer
			_, err := io.Copy(&buf, tr)
			require.NoError(t, err)
			lockData = buf.Bytes()
			break
		}
	}
	require.NotNil(t, lockHdr, "Chart.lock not found in archive")
	require.NotNil(t, lockData)

	// Validate the generated field in the YAML is the epoch (not the fixture's original timestamp).
	var lock struct {
		Generated string `yaml:"generated"`
	}
	err = yaml.Unmarshal(lockData, &lock)
	require.NoError(t, err)
	require.Equal(t, "2021-01-01T00:00:00Z", lock.Generated,
		"Chart.lock generated field must match SourceDateEpoch")

	// Tar header ModTime must also match.
	require.True(t, lockHdr.ModTime.Equal(epoch),
		"Chart.lock tar header ModTime must match SourceDateEpoch: got %v, want %v",
		lockHdr.ModTime, epoch)
}
