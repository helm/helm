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
	"os"
	"path"
	"testing"

	"github.com/Masterminds/semver/v3"
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
		_, err = w.Write([]byte(passphrase + "\n"))
		require.NoError(t, err)
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
				if err != tt.wantErr {
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
