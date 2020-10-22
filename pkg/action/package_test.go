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
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/Masterminds/semver/v3"

	"helm.sh/helm/v3/internal/test/ensure"
)

func TestPassphraseFileFetcher(t *testing.T) {
	secret := "secret"
	directory := ensure.TempFile(t, "passphrase-file", []byte(secret))
	defer os.RemoveAll(directory)

	fetcher, err := passphraseFileFetcher(path.Join(directory, "passphrase-file"), nil)
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
	defer os.RemoveAll(directory)

	fetcher, err := passphraseFileFetcher(path.Join(directory, "passphrase-file"), nil)
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
	directory := ensure.TempDir(t)
	defer os.RemoveAll(directory)

	stdin, err := ioutil.TempFile(directory, "non-existing")
	if err != nil {
		t.Fatal("Unable to create test file", err)
	}

	if _, err := passphraseFileFetcher("-", stdin); err == nil {
		t.Error("Expected passphraseFileFetcher returning an error")
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
