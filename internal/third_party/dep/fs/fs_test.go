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
)

func TestRenameWithFallback(t *testing.T) {
	dir := t.TempDir()

	if err := RenameWithFallback(filepath.Join(dir, "does_not_exists"), filepath.Join(dir, "dst")); err == nil {
		t.Fatal("expected an error for non existing file, but got nil")
	}

	srcpath := filepath.Join(dir, "src")

	if srcf, err := os.Create(srcpath); err != nil {
		t.Fatal(err)
	} else {
		srcf.Close()
	}

	if err := RenameWithFallback(srcpath, filepath.Join(dir, "dst")); err != nil {
		t.Fatal(err)
	}

	srcpath = filepath.Join(dir, "a")
	if err := os.MkdirAll(srcpath, 0777); err != nil {
		t.Fatal(err)
	}

	dstpath := filepath.Join(dir, "b")
	if err := os.MkdirAll(dstpath, 0777); err != nil {
		t.Fatal(err)
	}

	if err := RenameWithFallback(srcpath, dstpath); err == nil {
		t.Fatal("expected an error if dst is an existing directory, but got nil")
	}
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()

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
		if err := os.MkdirAll(dn, 0755); err != nil {
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
	if err := CopyDir(srcdir, destdir, CopyDirOptions{}); err != nil {
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

		got, err := os.ReadFile(fn)
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

func TestCopyDir_IgnoreDirectories(t *testing.T) {
	dir := t.TempDir()

	srcdir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a .git directory that should be ignored
	gitdir := filepath.Join(srcdir, ".git")
	if err := os.MkdirAll(gitdir, 0755); err != nil {
		t.Fatal(err)
	}
	gitFile := filepath.Join(gitdir, "config")
	if err := os.WriteFile(gitFile, []byte("git config"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a nested .git directory
	nestedDir := filepath.Join(srcdir, "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Add a regular file in subdir to ensure the directory itself gets copied
	nestedRegularFile := filepath.Join(nestedDir, "file.txt")
	if err := os.WriteFile(nestedRegularFile, []byte("nested regular file"), 0644); err != nil {
		t.Fatal(err)
	}
	nestedGitDir := filepath.Join(nestedDir, ".git")
	if err := os.MkdirAll(nestedGitDir, 0755); err != nil {
		t.Fatal(err)
	}
	nestedGitFile := filepath.Join(nestedGitDir, "config")
	if err := os.WriteFile(nestedGitFile, []byte("nested git config"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a regular directory that should NOT be ignored
	regulardir := filepath.Join(srcdir, "regular")
	if err := os.MkdirAll(regulardir, 0755); err != nil {
		t.Fatal(err)
	}
	regularFile := filepath.Join(regulardir, "file.txt")
	if err := os.WriteFile(regularFile, []byte("regular file"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a regular file at the root
	rootFile := filepath.Join(srcdir, "root.txt")
	if err := os.WriteFile(rootFile, []byte("root file"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create another directory to ignore (e.g., .vscode)
	vscodeDir := filepath.Join(srcdir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	vscodeFile := filepath.Join(vscodeDir, "settings.json")
	if err := os.WriteFile(vscodeFile, []byte("vscode settings"), 0644); err != nil {
		t.Fatal(err)
	}

	destdir := filepath.Join(dir, "dest")
	dirsToIgnore := []string{".git", ".vscode"}
	if err := CopyDir(srcdir, destdir, CopyDirOptions{DirsInSrcToIgnore: dirsToIgnore}); err != nil {
		t.Fatal(err)
	}

	// Verify .git directory was NOT copied
	destGitDir := filepath.Join(destdir, ".git")
	if _, err := os.Stat(destGitDir); err == nil {
		t.Fatalf("expected .git directory to be ignored, but it was copied to %s", destGitDir)
	}

	// Verify nested .git directory was NOT copied
	destNestedGitDir := filepath.Join(destdir, "subdir", ".git")
	if _, err := os.Stat(destNestedGitDir); err == nil {
		t.Fatalf("expected nested .git directory to be ignored, but it was copied to %s", destNestedGitDir)
	}

	// Verify subdir itself WAS copied (it contains a regular file)
	destSubdirFile := filepath.Join(destdir, "subdir", "file.txt")
	gotSubdir, err := os.ReadFile(destSubdirFile)
	if err != nil {
		t.Fatalf("expected nested regular file to be copied, but it was not found at %s: %v", destSubdirFile, err)
	}
	if string(gotSubdir) != "nested regular file" {
		t.Fatalf("expected file contents 'nested regular file', got %s", string(gotSubdir))
	}

	// Verify .vscode directory was NOT copied
	destVscodeDir := filepath.Join(destdir, ".vscode")
	if _, err := os.Stat(destVscodeDir); err == nil {
		t.Fatalf("expected .vscode directory to be ignored, but it was copied to %s", destVscodeDir)
	}

	// Verify regular directory WAS copied
	destRegularDir := filepath.Join(destdir, "regular")
	if _, err := os.Stat(destRegularDir); err != nil {
		t.Fatalf("expected regular directory to be copied, but it was not found at %s: %v", destRegularDir, err)
	}

	// Verify regular file WAS copied
	destRegularFile := filepath.Join(destdir, "regular", "file.txt")
	got, err := os.ReadFile(destRegularFile)
	if err != nil {
		t.Fatalf("expected regular file to be copied, but it was not found at %s: %v", destRegularFile, err)
	}
	if string(got) != "regular file" {
		t.Fatalf("expected file contents 'regular file', got %s", string(got))
	}

	// Verify root file WAS copied
	destRootFile := filepath.Join(destdir, "root.txt")
	got, err = os.ReadFile(destRootFile)
	if err != nil {
		t.Fatalf("expected root file to be copied, but it was not found at %s: %v", destRootFile, err)
	}
	if string(got) != "root file" {
		t.Fatalf("expected file contents 'root file', got %s", string(got))
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

	dir := t.TempDir()

	dstdir = filepath.Join(dir, "dst")
	if err := CopyDir(srcdir, dstdir, CopyDirOptions{}); err == nil {
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

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0755); err != nil {
		t.Fatal(err)
	}

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dstdir = filepath.Join(dir, "dst")
		return nil
	})
	defer cleanup()

	if err := CopyDir(srcdir, dstdir, CopyDirOptions{}); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
}

func TestCopyDirFail_SrcIsNotDir(t *testing.T) {
	var srcdir, dstdir string
	var err error

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if _, err = os.Create(srcdir); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")

	if err = CopyDir(srcdir, dstdir, CopyDirOptions{}); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}

	if err != errSrcNotDir {
		t.Fatalf("expected %v error for CopyDir(%s, %s), got %s", errSrcNotDir, srcdir, dstdir, err)
	}

}

func TestCopyDirFail_DstExists(t *testing.T) {
	var srcdir, dstdir string
	var err error

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if err = os.MkdirAll(srcdir, 0755); err != nil {
		t.Fatal(err)
	}

	dstdir = filepath.Join(dir, "dst")
	if err = os.MkdirAll(dstdir, 0755); err != nil {
		t.Fatal(err)
	}

	if err = CopyDir(srcdir, dstdir, CopyDirOptions{}); err == nil {
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

	dir := t.TempDir()

	srcdir = filepath.Join(dir, "src")
	if err := os.MkdirAll(srcdir, 0755); err != nil {
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

	if err = CopyDir(srcdir, dstdir, CopyDirOptions{}); err == nil {
		t.Fatalf("expected error for CopyDir(%s, %s), got none", srcdir, dstdir)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

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
	if err := CopyFile(srcf.Name(), destf); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(destf)
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

func TestCopyFileSymlink(t *testing.T) {
	tempdir := t.TempDir()

	testcases := map[string]string{
		filepath.Join("./testdata/symlinks/file-symlink"):         filepath.Join(tempdir, "dst-file"),
		filepath.Join("./testdata/symlinks/windows-file-symlink"): filepath.Join(tempdir, "windows-dst-file"),
		filepath.Join("./testdata/symlinks/invalid-symlink"):      filepath.Join(tempdir, "invalid-symlink"),
	}

	for symlink, dst := range testcases {
		t.Run(symlink, func(t *testing.T) {
			var err error
			if err = CopyFile(symlink, dst); err != nil {
				t.Fatalf("failed to copy symlink: %s", err)
			}

			var want, got string

			if runtime.GOOS == "windows" {
				// Creating symlinks on Windows require an additional permission
				// regular users aren't granted usually. So we copy the file
				// content as a fall back instead of creating a real symlink.
				srcb, err := os.ReadFile(symlink)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				dstb, err := os.ReadFile(dst)
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

	dir := t.TempDir()

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
	if err := CopyFile(srcf.Name(), fn); err == nil {
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
	t.Helper()
	dir := t.TempDir()

	subdir := filepath.Join(dir, "dir")

	cleanup := func() {
		if err := os.Chmod(subdir, 0777); err != nil {
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

	dir := t.TempDir()

	dirPath := filepath.Join(dir, "directory")
	if err := os.MkdirAll(dirPath, 0777); err != nil {
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
