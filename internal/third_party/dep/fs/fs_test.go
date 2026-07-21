/*
Copyright (c) for portions of fs_test.go are held by The Go Authors, 2016 and are provided under
the BSD license.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenameWithFallback(t *testing.T) {
	dir := t.TempDir()

	require.Error(t, RenameWithFallback(filepath.Join(dir, "does_not_exists"), filepath.Join(dir, "dst")), "expected an error for non existing file, but got nil")

	srcpath := filepath.Join(dir, "src")

	srcf, err := os.Create(srcpath)
	require.NoError(t, err)
	srcf.Close()

	require.NoError(t, RenameWithFallback(srcpath, filepath.Join(dir, "dst")))

	srcpath = filepath.Join(dir, "a")
	require.NoError(t, os.MkdirAll(srcpath, 0o777))

	dstpath := filepath.Join(dir, "b")
	require.NoError(t, os.MkdirAll(dstpath, 0o777))
	require.Error(t, RenameWithFallback(srcpath, dstpath), "expected an error if dst is an existing directory, but got nil")
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()

	srcdir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcdir, 0o755))

	files := []struct {
		path     string
		contents string
		fi       os.FileInfo
	}{
		{path: "myfile", contents: "hello world"},
		{path: filepath.Join("subdir", "file"), contents: "subdir file"},
	}

	// Create structure indicated in 'files'
	for i, file := range files {
		fn := filepath.Join(srcdir, file.path)
		dn := filepath.Dir(fn)
		require.NoError(t, os.MkdirAll(dn, 0o755))

		fh, err := os.Create(fn)
		require.NoError(t, err)

		_, err = fh.WriteString(file.contents)
		require.NoError(t, err)
		fh.Close()

		files[i].fi, err = os.Stat(fn)
		require.NoError(t, err)
	}

	destdir := filepath.Join(dir, "dest")
	require.NoError(t, CopyDir(srcdir, destdir))

	// Compare copy against structure indicated in 'files'
	for _, file := range files {
		fn := filepath.Join(srcdir, file.path)
		dn := filepath.Dir(fn)
		dirOK, err := IsDir(dn)
		require.NoError(t, err)
		require.Truef(t, dirOK, "expected %s to be a directory", dn)

		got, err := os.ReadFile(fn)
		require.NoError(t, err)

		require.Equalf(t, file.contents, string(got), "expected: %s, got: %s", file.contents, string(got))

		gotinfo, err := os.Stat(fn)
		require.NoError(t, err)

		require.Equalf(t, file.fi.Mode(), gotinfo.Mode(), "expected %s: %#v\n to be the same mode as %s: %#v",
			file.path, file.fi.Mode(), fn, gotinfo.Mode())
	}
}

func TestCopyDirFail_SrcInaccessible(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. Skipping this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	var currentUID = os.Getuid()

	if currentUID == 0 {
		// Skipping if root, because all files are accessible
		t.Skip("Skipping for root user")
	}

	var srcdir, dstdir string

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		srcdir = filepath.Join(dir, "src")
		return os.MkdirAll(srcdir, 0o755)
	})
	defer cleanup()

	dir := t.TempDir()

	dstdir = filepath.Join(dir, "dst")
	assert.Errorf(t, CopyDir(srcdir, dstdir), "expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
}

func TestCopyDirFail_DstInaccessible(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. Skipping this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	var currentUID = os.Getuid()

	if currentUID == 0 {
		// Skipping if root, because all files are accessible
		t.Skip("Skipping for root user")
	}

	var srcdir, dstdir string

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcdir, 0o755))

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dstdir = filepath.Join(dir, "dst")
		return nil
	})
	defer cleanup()

	assert.Errorf(t, CopyDir(srcdir, dstdir), "expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
}

func TestCopyDirFail_SrcIsNotDir(t *testing.T) {
	var srcdir, dstdir string
	var err error

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	_, err = os.Create(srcdir)
	require.NoError(t, err)

	dstdir = filepath.Join(dir, "dst")

	require.ErrorIsf(t, CopyDir(srcdir, dstdir), errSrcNotDir, "expected %v error for CopyDir(%s, %s)", errSrcNotDir, srcdir, dstdir)
}

func TestCopyDirFail_DstExists(t *testing.T) {
	var srcdir, dstdir string

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcdir, 0o755))

	dstdir = filepath.Join(dir, "dst")
	require.NoError(t, os.MkdirAll(dstdir, 0o755))
	require.ErrorIs(t, CopyDir(srcdir, dstdir), errDstExist, "expected %v error for CopyDir(%s, %s)", errDstExist, srcdir, dstdir)
}

func TestCopyDirFailOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. os.Chmod(..., 0222) below is not
		// enough for the file to be readonly, and os.Chmod(...,
		// 0000) returns an invalid argument error. Skipping
		// this until a compatible implementation is
		// provided.
		t.Skip("skipping on windows")
	}

	var currentUID = os.Getuid()

	if currentUID == 0 {
		// Skipping if root, because all files are accessible
		t.Skip("Skipping for root user")
	}

	var srcdir, dstdir string

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcdir, 0o755))

	srcfn := filepath.Join(srcdir, "file")
	srcf, err := os.Create(srcfn)
	require.NoError(t, err)
	srcf.Close()

	// setup source file so that it cannot be read
	require.NoError(t, os.Chmod(srcfn, 0o222))

	dstdir = filepath.Join(dir, "dst")
	assert.Errorf(t, CopyDir(srcdir, dstdir), "expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	srcf, err := os.Create(filepath.Join(dir, "srcfile"))
	require.NoError(t, err)

	want := "hello world"
	_, err = srcf.WriteString(want)
	require.NoError(t, err)
	srcf.Close()

	destf := filepath.Join(dir, "destf")
	require.NoError(t, CopyFile(srcf.Name(), destf))

	got, err := os.ReadFile(destf)
	require.NoError(t, err)

	require.Equalf(t, want, string(got), "expected: %s, got: %s", want, string(got))

	wantinfo, err := os.Stat(srcf.Name())
	require.NoError(t, err)

	gotinfo, err := os.Stat(destf)
	require.NoError(t, err)

	assert.Equalf(t, wantinfo.Mode(), gotinfo.Mode(), "expected %s: %#v\n to be the same mode as %s: %#v", srcf.Name(), wantinfo.Mode(), destf, gotinfo.Mode())
}

func TestCopyFileSymlink(t *testing.T) {
	tempdir := t.TempDir()

	testcases := map[string]string{
		filepath.Join(".", "testdata", "symlinks", "file-symlink"):         filepath.Join(tempdir, "dst-file"),
		filepath.Join(".", "testdata", "symlinks", "windows-file-symlink"): filepath.Join(tempdir, "windows-dst-file"),
		filepath.Join(".", "testdata", "symlinks", "invalid-symlink"):      filepath.Join(tempdir, "invalid-symlink"),
	}

	for symlink, dst := range testcases {
		t.Run(symlink, func(t *testing.T) {
			var err error
			require.NoErrorf(t, CopyFile(symlink, dst), "failed to copy symlink")

			var want, got string

			if runtime.GOOS == "windows" {
				// Creating symlinks on Windows require an additional permission
				// regular users aren't granted usually. So we copy the file
				// content as a fall back instead of creating a real symlink.
				srcb, err := os.ReadFile(symlink)
				require.NoError(t, err)
				dstb, err := os.ReadFile(dst)
				require.NoError(t, err)

				want = string(srcb)
				got = string(dstb)
			} else {
				want, err = os.Readlink(symlink)
				require.NoError(t, err)

				got, err = os.Readlink(dst)
				require.NoErrorf(t, err, "could not resolve symlink")
			}

			require.Equalf(t, want, got, "resolved path is incorrect. expected %s, got %s", want, got)
		})
	}
}

func TestCopyFileFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. Skipping this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	var currentUID = os.Getuid()

	if currentUID == 0 {
		// Skipping if root, because all files are accessible
		t.Skip("Skipping for root user")
	}

	dir := t.TempDir()

	srcf, err := os.Create(filepath.Join(dir, "srcfile"))
	require.NoError(t, err)
	srcf.Close()

	var dstdir string

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dstdir = filepath.Join(dir, "dir")
		return os.Mkdir(dstdir, 0o777)
	})
	defer cleanup()

	fn := filepath.Join(dstdir, "file")
	require.Errorf(t, CopyFile(srcf.Name(), fn), "expected error for %s, got none", fn)
}

// setupInaccessibleDir creates a temporary location with a single
// directory in it, in such a way that directory is not accessible
// after this function returns.
//
// op is called with the directory as argument, so that it can create
// files or other test artifacts.
//
// If setupInaccessibleDir fails in its preparation, or op fails, t.Fatal
// will be invoked.
//
// This function returns a cleanup function that removes all the temporary
// files this function creates. It is the caller's responsibility to call
// this function before the test is done running, whether there's an error or not.
func setupInaccessibleDir(t *testing.T, op func(dir string) error) func() {
	t.Helper()
	dir := t.TempDir()

	subdir := filepath.Join(dir, "dir")

	cleanup := func() {
		assert.NoError(t, os.Chmod(subdir, 0o777))
	}

	if err := os.Mkdir(subdir, 0o777); err != nil {
		cleanup()
		t.Fatal(err)
		return nil
	}

	if err := op(subdir); err != nil {
		cleanup()
		t.Fatal(err)
		return nil
	}

	if err := os.Chmod(subdir, 0o666); err != nil {
		cleanup()
		t.Fatal(err)
		return nil
	}

	return cleanup
}

func TestIsDir(t *testing.T) {
	var currentUID = os.Getuid()

	if currentUID == 0 {
		// Skipping if root, because all files are accessible
		t.Skip("Skipping for root user")
	}

	wd, err := os.Getwd()
	require.NoError(t, err)

	var dn string

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dn = filepath.Join(dir, "dir")
		return os.Mkdir(dn, 0o777)
	})
	defer cleanup()

	tests := map[string]struct {
		exists bool
		err    bool
	}{
		wd:                            {true, false},
		filepath.Join(wd, "testdata"): {true, false},
		filepath.Join(wd, "main.go"):  {false, true},
		filepath.Join(wd, "this_file_does_not_exist.thing"): {false, true},
		dn: {false, true},
	}

	if runtime.GOOS == "windows" {
		// This test doesn't work on Microsoft Windows because
		// of the differences in how file permissions are
		// implemented. For this to work, the directory where
		// the directory exists should be inaccessible.
		delete(tests, dn)
	}

	for f, want := range tests {
		t.Run(f, func(t *testing.T) {
			got, err := IsDir(f)

			if want.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equalf(t, want.exists, got, "expected %t for %s, got %t", want.exists, f, got)
		})
	}
}

func TestIsSymlink(t *testing.T) {
	var currentUID = os.Getuid()

	if currentUID == 0 {
		// Skipping if root, because all files are accessible
		t.Skip("Skipping for root user")
	}

	dir := t.TempDir()

	dirPath := filepath.Join(dir, "directory")
	require.NoError(t, os.MkdirAll(dirPath, 0o777))

	filePath := filepath.Join(dir, "file")
	f, err := os.Create(filePath)
	require.NoError(t, err)
	f.Close()

	dirSymlink := filepath.Join(dir, "dirSymlink")
	fileSymlink := filepath.Join(dir, "fileSymlink")

	require.NoError(t, os.Symlink(dirPath, dirSymlink))
	require.NoError(t, os.Symlink(filePath, fileSymlink))

	var (
		inaccessibleFile    string
		inaccessibleSymlink string
	)

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		inaccessibleFile = filepath.Join(dir, "file")
		if fh, err := os.Create(inaccessibleFile); err != nil {
			return err
		} else if err := fh.Close(); err != nil {
			return err
		}

		inaccessibleSymlink = filepath.Join(dir, "symlink")
		return os.Symlink(inaccessibleFile, inaccessibleSymlink)
	})
	defer cleanup()

	tests := map[string]struct{ expected, err bool }{
		dirPath:             {false, false},
		filePath:            {false, false},
		dirSymlink:          {true, false},
		fileSymlink:         {true, false},
		inaccessibleFile:    {false, true},
		inaccessibleSymlink: {false, true},
	}

	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in Windows. Skipping
		// these cases until a compatible implementation is provided.
		delete(tests, inaccessibleFile)
		delete(tests, inaccessibleSymlink)
	}

	for path, want := range tests {
		got, err := IsSymlink(path)
		if want.err {
			require.Error(t, err, "expected an error")
		} else {
			require.NoError(t, err, "expected no error")
		}
		assert.Equalf(t, want.expected, got, "expected %t for %s, got %t", want.expected, path, got)
	}
}
