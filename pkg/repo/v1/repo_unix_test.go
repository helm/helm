//go:build !windows

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

package repo

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileNewFileHonorsUmask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repositories.yaml")
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	f := NewFile()
	f.Add(&Entry{Name: "stable", URL: "https://example.com/stable/charts"})
	require.NoError(t, f.WriteFile(path, 0o644))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equalf(t, os.FileMode(0o600), info.Mode().Perm(), "perm=0644 with umask 0077 must yield 0600")
}
