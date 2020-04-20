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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	mu sync.Mutex
)

func TestRenameWithFallback(t *testing.T) {
	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err = RenameWithFallback(filepath.Join(dir, "does_not_exists"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("expected an error for non existing file, but got nil")
	}

	srcpath := filepath.Join(dir, "src")

	if srcf, err := os.Create(srcpath); err != nil {
		t.Fatal(err)
	} else {
		srcf.Close()
	}

	if err = RenameWithFallback(srcpath, filepath.Join(dir, "dst")); err != nil {
		t.Fatal(err)
	}

	srcpath = filepath.Join(dir, "a")
	if err = os.MkdirAll(srcpath, 0777); err != nil {
		t.Fatal(err)
	}

	dstpath := filepath.Join(dir, "b")
	if err = os.MkdirAll(dstpath, 0777); err != nil {
		t.Fatal(err)
	}

	if err = RenameWithFallback(srcpath, dstpath); err == nil {
		t.Fatal("expected an error if dst is an existing directory, but got nil")
	}
}

func TestCopyDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	srcdir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0755); err != nil {
		t.Fatal(err)
	}

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
		if err = os.MkdirAll(dn, 0755); err != nil {
			t.Fatal(err)
		}

		fh, err := os.Create(fn)
		if err != nil {
			t.Fatal(err)
		}

		if _, err = fh.Write([]byte(file.contents)); err != nil {
			t.Fatal(err)
		}
		fh.Close()

		files[i].fi, err = os.Stat(fn)
		if err != nil {
			t.Fatal(err)
		}
	}

	destdir := filepath.Join(dir, "dest")
	if err := CopyDir(srcdir, destdir); err != nil {
		t.Fatal(err)
	}

	// Compare copy against structure indicated in 'files'
	for _, file := range files {
		fn := filepath.Join(srcdir, file.path)
		dn := filepath.Dir(fn)
		dirOK, err := IsDir(dn)
		if err != nil {
			t.Fatal(err)
		}
		if !dirOK {
			t.Fatalf("expected %s to be a directory", dn)
		}

		got, err := ioutil.ReadFile(fn)
		if err != nil {
			t.Fatal(err)
		}

		if file.contents != string(got) {
			t.Fatalf("expected: %s, got: %s", file.contents, string(got))
		}

		gotinfo, err := os.Stat(fn)
		if err != nil {
			t.Fatal(err)
		}

		if file.fi.Mode() != gotinfo.Mode() {
			t.Fatalf("expected %s: %#v\n to be the same mode as %s: %#v",
				file.path, file.fi.Mode(), fn, gotinfo.Mode())
		}
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
		return os.MkdirAll(srcdir, 0755)
	})
	defer cleanup()

	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dstdir = filepath.Join(dir, "dst")
	if err = CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
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

	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	srcdir = filepath.Join(dir, "src")
	if err = os.MkdirAll(srcdir, 0755); err != nil {
		t.Fatal(err)
	}

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dstdir = filepath.Join(dir, "dst")
		return nil
	})
	defer cleanup()

	if err := CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
}

func TestCopyDirFail_SrcIsNotDir(t *testing.T) {
	var srcdir, dstdir string

	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	srcdir = filepath.Join(dir, "src")
	if _, err = os.Create(srcdir); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")

	if err = CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}

	if err != errSrcNotDir {
		t.Fatalf("expected %v error for CopyDir(%s, %s), got %s", errSrcNotDir, srcdir, dstdir, err)
	}

}

func TestCopyDirFail_DstExists(t *testing.T) {
	var srcdir, dstdir string

	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	srcdir = filepath.Join(dir, "src")
	if err = os.MkdirAll(srcdir, 0755); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")
	if err = os.MkdirAll(dstdir, 0755); err != nil {
		t.Fatal(err)
	}

	if err = CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}

	if err != errDstExist {
		t.Fatalf("expected %v error for CopyDir(%s, %s), got %s", errDstExist, srcdir, dstdir, err)
	}
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

	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	srcdir = filepath.Join(dir, "src")
	if err = os.MkdirAll(srcdir, 0755); err != nil {
		t.Fatal(err)
	}

	srcfn := filepath.Join(srcdir, "file")
	srcf, err := os.Create(srcfn)
	if err != nil {
		t.Fatal(err)
	}
	srcf.Close()

	// setup source file so that it cannot be read
	if err = os.Chmod(srcfn, 0222); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")

	if err = CopyDir(srcdir, dstdir); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
}

func TestCopyFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	srcf, err := os.Create(filepath.Join(dir, "srcfile"))
	if err != nil {
		t.Fatal(err)
	}

	want := "hello world"
	if _, err := srcf.Write([]byte(want)); err != nil {
		t.Fatal(err)
	}
	srcf.Close()

	destf := filepath.Join(dir, "destf")
	if err := copyFile(srcf.Name(), destf); err != nil {
		t.Fatal(err)
	}

	got, err := ioutil.ReadFile(destf)
	if err != nil {
		t.Fatal(err)
	}

	if want != string(got) {
		t.Fatalf("expected: %s, got: %s", want, string(got))
	}

	wantinfo, err := os.Stat(srcf.Name())
	if err != nil {
		t.Fatal(err)
	}

	gotinfo, err := os.Stat(destf)
	if err != nil {
		t.Fatal(err)
	}

	if wantinfo.Mode() != gotinfo.Mode() {
		t.Fatalf("expected %s: %#v\n to be the same mode as %s: %#v", srcf.Name(), wantinfo.Mode(), destf, gotinfo.Mode())
	}
}

func cleanUpDir(dir string) {
	// NOTE(mattn): It seems that sometimes git.exe is not dead
	// when cleanUpDir() is called. But we do not know any way to wait for it.
	if runtime.GOOS == "windows" {
		mu.Lock()
		exec.Command(`taskkill`, `/F`, `/IM`, `git.exe`).Run()
		mu.Unlock()
	}
	if dir != "" {
		os.RemoveAll(dir)
	}
}

func TestCopyFileSymlink(t *testing.T) {
	var tempdir, err = ioutil.TempDir("", "gotest")

	if err != nil {
		t.Fatalf("failed to create directory: %s", err)
	}

	defer cleanUpDir(tempdir)

	testcases := map[string]string{
		filepath.Join("./testdata/symlinks/file-symlink"):         filepath.Join(tempdir, "dst-file"),
		filepath.Join("./testdata/symlinks/windows-file-symlink"): filepath.Join(tempdir, "windows-dst-file"),
		filepath.Join("./testdata/symlinks/invalid-symlink"):      filepath.Join(tempdir, "invalid-symlink"),
	}

	for symlink, dst := range testcases {
		t.Run(symlink, func(t *testing.T) {
			var err error
			if err = copyFile(symlink, dst); err != nil {
				t.Fatalf("failed to copy symlink: %s", err)
			}

			var want, got string

			if runtime.GOOS == "windows" {
				// Creating symlinks on Windows require an additional permission
				// regular users aren't granted usually. So we copy the file
				// content as a fall back instead of creating a real symlink.
				srcb, err := ioutil.ReadFile(symlink)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				dstb, err := ioutil.ReadFile(dst)
				if err != nil {
					t.Fatalf("%+v", err)
				}

				want = string(srcb)
				got = string(dstb)
			} else {
				want, err = os.Readlink(symlink)
				if err != nil {
					t.Fatalf("%+v", err)
				}

				got, err = os.Readlink(dst)
				if err != nil {
					t.Fatalf("could not resolve symlink: %s", err)
				}
			}

			if want != got {
				t.Fatalf("resolved path is incorrect. expected %s, got %s", want, got)
			}
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

	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	srcf, err := os.Create(filepath.Join(dir, "srcfile"))
	if err != nil {
		t.Fatal(err)
	}
	srcf.Close()

	var dstdir string

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dstdir = filepath.Join(dir, "dir")
		return os.Mkdir(dstdir, 0777)
	})
	defer cleanup()

	fn := filepath.Join(dstdir, "file")
	if err := copyFile(srcf.Name(), fn); err == nil {
		t.Fatalf("expected error for %s, got none", fn)
	}
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
	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
		return nil // keep compiler happy
	}

	subdir := filepath.Join(dir, "dir")

	cleanup := func() {
		if err := os.Chmod(subdir, 0777); err != nil {
			t.Error(err)
		}
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	}

	if err := os.Mkdir(subdir, 0777); err != nil {
		cleanup()
		t.Fatal(err)
		return nil
	}

	if err := op(subdir); err != nil {
		cleanup()
		t.Fatal(err)
		return nil
	}

	if err := os.Chmod(subdir, 0666); err != nil {
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
	if err != nil {
		t.Fatal(err)
	}

	var dn string

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dn = filepath.Join(dir, "dir")
		return os.Mkdir(dn, 0777)
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
		got, err := IsDir(f)
		if err != nil && !want.err {
			t.Fatalf("expected no error, got %v", err)
		}

		if got != want.exists {
			t.Fatalf("expected %t for %s, got %t", want.exists, f, got)
		}
	}
}

func TestIsSymlink(t *testing.T) {

	var currentUID = os.Getuid()

	if currentUID == 0 {
		// Skipping if root, because all files are accessible
		t.Skip("Skipping for root user")
	}

	dir, err := ioutil.TempDir("", "helm-tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dirPath := filepath.Join(dir, "directory")
	if err = os.MkdirAll(dirPath, 0777); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(dir, "file")
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	dirSymlink := filepath.Join(dir, "dirSymlink")
	fileSymlink := filepath.Join(dir, "fileSymlink")

	if err = os.Symlink(dirPath, dirSymlink); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filePath, fileSymlink); err != nil {
		t.Fatal(err)
	}

	var (
		inaccessibleFile    string
		inaccessibleSymlink string
	)

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		inaccessibleFile = filepath.Join(dir, "file")
		if fh, err := os.Create(inaccessibleFile); err != nil {
			return err
		} else if err = fh.Close(); err != nil {
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
		if err != nil {
			if !want.err {
				t.Errorf("expected no error, got %v", err)
			}
		}

		if got != want.expected {
			t.Errorf("expected %t for %s, got %t", want.expected, path, got)
		}
	}
}
