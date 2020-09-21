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

	"helm.sh/helm/v3/internal/test/ensure"
	"helm.sh/helm/v3/pkg/chart"
)

func TestSetVersion(t *testing.T) {
	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "prow",
			Version: "0.0.1",
		},
	}
	expect := "1.2.3-beta.5"
	if err := setVersion(c, expect); err != nil {
		t.Fatal(err)
	}

	if c.Metadata.Version != expect {
		t.Errorf("Expected %q, got %q", expect, c.Metadata.Version)
	}

	if err := setVersion(c, "monkeyface"); err == nil {
		t.Error("Expected bogus version to return an error.")
	}
}

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
