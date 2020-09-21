// +build !windows

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

package main

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckPerms(t *testing.T) {
	// NOTE(bacongobbler): have to open a new file handler here as the default os.Sterr cannot be read from
	stderr, err := os.Open("/dev/stderr")
	if err != nil {
		t.Fatalf("could not open /dev/stderr for reading: %s", err)
	}
	defer stderr.Close()
	reader := bufio.NewReader(stderr)

	tdir, err := ioutil.TempDir("", "helmtest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)
	tfile := filepath.Join(tdir, "testconfig")
	fh, err := os.OpenFile(tfile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0440)
	if err != nil {
		t.Errorf("Failed to create temp file: %s", err)
	}

	tconfig := settings.KubeConfig
	settings.KubeConfig = tfile
	defer func() { settings.KubeConfig = tconfig }()

	checkPerms()
	text, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("could not read from stderr: %s", err)
	}
	expectPrefix := "WARNING: Kubernetes configuration file is group-readable. This is insecure. Location:"
	if !strings.HasPrefix(text, expectPrefix) {
		t.Errorf("Expected to get a warning for group perms. Got %q", text)
	}

	if err := fh.Chmod(0404); err != nil {
		t.Errorf("Could not change mode on file: %s", err)
	}
	checkPerms()
	text, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("could not read from stderr: %s", err)
	}
	expectPrefix = "WARNING: Kubernetes configuration file is world-readable. This is insecure. Location:"
	if !strings.HasPrefix(text, expectPrefix) {
		t.Errorf("Expected to get a warning for world perms. Got %q", text)
	}
}
