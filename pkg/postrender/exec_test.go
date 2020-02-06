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

package postrender

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v3/internal/test/ensure"
)

const testingScript = `#!/bin/sh
sed s/FOOTEST/BARTEST/g <&0
`

func TestGetFullPath(t *testing.T) {
	is := assert.New(t)
	t.Run("full path resolves correctly", func(t *testing.T) {
		testpath, cleanup := setupTestingScript(t)
		defer cleanup()

		fullPath, err := getFullPath(testpath)
		is.NoError(err)
		is.Equal(testpath, fullPath)
	})

	t.Run("relative path resolves correctly", func(t *testing.T) {
		testpath, cleanup := setupTestingScript(t)
		defer cleanup()

		currentDir, err := os.Getwd()
		require.NoError(t, err)
		relative, err := filepath.Rel(currentDir, testpath)
		require.NoError(t, err)
		fullPath, err := getFullPath(relative)
		is.NoError(err)
		is.Equal(testpath, fullPath)
	})

	t.Run("binary in PATH resolves correctly", func(t *testing.T) {
		testpath, cleanup := setupTestingScript(t)
		defer cleanup()

		realPath := os.Getenv("PATH")
		os.Setenv("PATH", filepath.Dir(testpath))
		defer func() {
			os.Setenv("PATH", realPath)
		}()

		fullPath, err := getFullPath(filepath.Base(testpath))
		is.NoError(err)
		is.Equal(testpath, fullPath)
	})

	// NOTE(thomastaylor312): See note in getFullPath for more details why this
	// is here

	// t.Run("binary in plugin path resolves correctly", func(t *testing.T) {
	// 	testpath, cleanup := setupTestingScript(t)
	// 	defer cleanup()

	// 	realPath := os.Getenv("HELM_PLUGINS")
	// 	os.Setenv("HELM_PLUGINS", filepath.Dir(testpath))
	// 	defer func() {
	// 		os.Setenv("HELM_PLUGINS", realPath)
	// 	}()

	// 	fullPath, err := getFullPath(filepath.Base(testpath))
	// 	is.NoError(err)
	// 	is.Equal(testpath, fullPath)
	// })

	// t.Run("binary in multiple plugin paths resolves correctly", func(t *testing.T) {
	// 	testpath, cleanup := setupTestingScript(t)
	// 	defer cleanup()

	// 	realPath := os.Getenv("HELM_PLUGINS")
	// 	os.Setenv("HELM_PLUGINS", filepath.Dir(testpath)+string(os.PathListSeparator)+"/another/dir")
	// 	defer func() {
	// 		os.Setenv("HELM_PLUGINS", realPath)
	// 	}()

	// 	fullPath, err := getFullPath(filepath.Base(testpath))
	// 	is.NoError(err)
	// 	is.Equal(testpath, fullPath)
	// })
}

func TestExecRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the actual Run test uses a basic sed example, so skip this test on windows
		t.Skip("skipping on windows")
	}
	is := assert.New(t)
	testpath, cleanup := setupTestingScript(t)
	defer cleanup()

	renderer, err := NewExec(testpath)
	require.NoError(t, err)

	output, err := renderer.Run(bytes.NewBufferString("FOOTEST"))
	is.NoError(err)
	is.Contains(output.String(), "BARTEST")
}

func setupTestingScript(t *testing.T) (filepath string, cleanup func()) {
	t.Helper()

	tempdir := ensure.TempDir(t)

	f, err := ioutil.TempFile(tempdir, "post-render-test.sh")
	if err != nil {
		t.Fatalf("unable to create tempfile for testing: %s", err)
	}

	_, err = f.WriteString(testingScript)
	if err != nil {
		t.Fatalf("unable to write tempfile for testing: %s", err)
	}

	err = f.Chmod(0755)
	if err != nil {
		t.Fatalf("unable to make tempfile executable for testing: %s", err)
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("unable to close tempfile after writing: %s", err)
	}

	return f.Name(), func() {
		os.RemoveAll(tempdir)
	}
}
