// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/golang/dep/internal/test"
	"github.com/pkg/errors"
)

// This function tests HadFilepathPrefix. It should test it on both case
// sensitive and insensitive situations. However, the only reliable way to test
// case-insensitive behaviour is if using case-insensitive filesystem.  This
// cannot be guaranteed in an automated test. Therefore, the behaviour of the
// tests is not to test case sensitivity on *nix and to assume that Windows is
// case-insensitive. Please see link below for some background.
//
// https://superuser.com/questions/266110/how-do-you-make-windows-7-fully-case-sensitive-with-respect-to-the-filesystem
//
// NOTE: NTFS can be made case-sensitive. However many Windows programs,
// including Windows Explorer do not handle gracefully multiple files that
// differ only in capitalization. It is possible that this can cause these tests
// to fail on some setups.
func TestHasFilepathPrefix(t *testing.T) {
	dir, err := ioutil.TempDir("", "dep")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// dir2 is the same as dir but with different capitalization on Windows to
	// test case insensitivity
	var dir2 string
	if runtime.GOOS == "windows" {
		dir = strings.ToLower(dir)
		dir2 = strings.ToUpper(dir)
	} else {
		dir2 = dir
	}

	// For testing trailing and repeated separators
	sep := string(os.PathSeparator)

	cases := []struct {
		path   string
		prefix string
		want   bool
	}{
		{filepath.Join(dir, "a", "b"), filepath.Join(dir2), true},
		{filepath.Join(dir, "a", "b"), dir2 + sep + sep + "a", true},
		{filepath.Join(dir, "a", "b"), filepath.Join(dir2, "a") + sep, true},
		{filepath.Join(dir, "a", "b") + sep, filepath.Join(dir2), true},
		{dir + sep + sep + filepath.Join("a", "b"), filepath.Join(dir2, "a"), true},
		{filepath.Join(dir, "a", "b"), filepath.Join(dir2, "a"), true},
		{filepath.Join(dir, "a", "b"), filepath.Join(dir2, "a", "b"), true},
		{filepath.Join(dir, "a", "b"), filepath.Join(dir2, "c"), false},
		{filepath.Join(dir, "a", "b"), filepath.Join(dir2, "a", "d", "b"), false},
		{filepath.Join(dir, "a", "b"), filepath.Join(dir2, "a", "b2"), false},
		{filepath.Join(dir), filepath.Join(dir2, "a", "b"), false},
		{filepath.Join(dir, "ab"), filepath.Join(dir2, "a", "b"), false},
		{filepath.Join(dir, "ab"), filepath.Join(dir2, "a"), false},
		{filepath.Join(dir, "123"), filepath.Join(dir2, "123"), true},
		{filepath.Join(dir, "123"), filepath.Join(dir2, "1"), false},
		{filepath.Join(dir, "⌘"), filepath.Join(dir2, "⌘"), true},
		{filepath.Join(dir, "a"), filepath.Join(dir2, "⌘"), false},
		{filepath.Join(dir, "⌘"), filepath.Join(dir2, "a"), false},
	}

	for _, c := range cases {
		if err := os.MkdirAll(c.path, 0755); err != nil {
			t.Fatal(err)
		}

		if err = os.MkdirAll(c.prefix, 0755); err != nil {
			t.Fatal(err)
		}

		got, err := HasFilepathPrefix(c.path, c.prefix)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if c.want != got {
			t.Fatalf("dir: %q, prefix: %q, expected: %v, got: %v", c.path, c.prefix, c.want, got)
		}
	}
}

// This function tests HadFilepathPrefix. It should test it on both case
// sensitive and insensitive situations. However, the only reliable way to test
// case-insensitive behaviour is if using case-insensitive filesystem.  This
// cannot be guaranteed in an automated test. Therefore, the behaviour of the
// tests is not to test case sensitivity on *nix and to assume that Windows is
// case-insensitive. Please see link below for some background.
//
// https://superuser.com/questions/266110/how-do-you-make-windows-7-fully-case-sensitive-with-respect-to-the-filesystem
//
// NOTE: NTFS can be made case-sensitive. However many Windows programs,
// including Windows Explorer do not handle gracefully multiple files that
// differ only in capitalization. It is possible that this can cause these tests
// to fail on some setups.
func TestHasFilepathPrefix_Files(t *testing.T) {
	dir, err := ioutil.TempDir("", "dep")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// dir2 is the same as dir but with different capitalization on Windows to
	// test case insensitivity
	var dir2 string
	if runtime.GOOS == "windows" {
		dir = strings.ToLower(dir)
		dir2 = strings.ToUpper(dir)
	} else {
		dir2 = dir
	}

	existingFile := filepath.Join(dir, "exists")
	if err = os.MkdirAll(existingFile, 0755); err != nil {
		t.Fatal(err)
	}

	nonExistingFile := filepath.Join(dir, "does_not_exists")

	cases := []struct {
		path   string
		prefix string
		want   bool
		err    bool
	}{
		{existingFile, filepath.Join(dir2), true, false},
		{nonExistingFile, filepath.Join(dir2), false, true},
	}

	for _, c := range cases {
		got, err := HasFilepathPrefix(c.path, c.prefix)
		if err != nil && !c.err {
			t.Fatalf("unexpected error: %s", err)
		}
		if c.want != got {
			t.Fatalf("dir: %q, prefix: %q, expected: %v, got: %v", c.path, c.prefix, c.want, got)
		}
	}
}

func TestEquivalentPaths(t *testing.T) {
	h := test.NewHelper(t)
	h.TempDir("dir")
	h.TempDir("dir2")

	h.TempFile("file", "")
	h.TempFile("file2", "")

	h.TempDir("DIR")
	h.TempFile("FILE", "")

	testcases := []struct {
		p1, p2                   string
		caseSensitiveEquivalent  bool
		caseInensitiveEquivalent bool
		err                      bool
	}{
		{h.Path("dir"), h.Path("dir"), true, true, false},
		{h.Path("file"), h.Path("file"), true, true, false},
		{h.Path("dir"), h.Path("dir2"), false, false, false},
		{h.Path("file"), h.Path("file2"), false, false, false},
		{h.Path("dir"), h.Path("file"), false, false, false},
		{h.Path("dir"), h.Path("DIR"), false, true, false},
		{strings.ToLower(h.Path("dir")), strings.ToUpper(h.Path("dir")), false, true, true},
	}

	caseSensitive, err := IsCaseSensitiveFilesystem(h.Path("dir"))
	if err != nil {
		t.Fatal("unexpcted error:", err)
	}

	for _, tc := range testcases {
		got, err := EquivalentPaths(tc.p1, tc.p2)
		if err != nil && !tc.err {
			t.Error("unexpected error:", err)
		}
		if caseSensitive {
			if tc.caseSensitiveEquivalent != got {
				t.Errorf("expected EquivalentPaths(%q, %q) to be %t on case-sensitive filesystem, got %t", tc.p1, tc.p2, tc.caseSensitiveEquivalent, got)
			}
		} else {
			if tc.caseInensitiveEquivalent != got {
				t.Errorf("expected EquivalentPaths(%q, %q) to be %t on case-insensitive filesystem, got %t", tc.p1, tc.p2, tc.caseInensitiveEquivalent, got)
			}
		}
	}
}

func TestRenameWithFallback(t *testing.T) {
	dir, err := ioutil.TempDir("", "dep")
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

func TestIsCaseSensitiveFilesystem(t *testing.T) {
	isLinux := runtime.GOOS == "linux"
	isWindows := runtime.GOOS == "windows"
	isMacOS := runtime.GOOS == "darwin"

	if !isLinux && !isWindows && !isMacOS {
		t.Skip("Run this test on Windows, Linux and macOS only")
	}

	dir, err := ioutil.TempDir("", "TestCaseSensitivity")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var want bool
	if isLinux {
		want = true
	} else {
		want = false
	}

	got, err := IsCaseSensitiveFilesystem(dir)

	if err != nil {
		t.Fatalf("unexpected error message: \n\t(GOT) %+v", err)
	}

	if want != got {
		t.Fatalf("unexpected value returned: \n\t(GOT) %t\n\t(WNT) %t", got, want)
	}
}

func TestReadActualFilenames(t *testing.T) {
	// We are trying to skip this test on file systems which are case-sensiive. We could
	// have used `fs.IsCaseSensitiveFilesystem` for this check. However, the code we are
	// testing also relies on `fs.IsCaseSensitiveFilesystem`. So a bug in
	// `fs.IsCaseSensitiveFilesystem` could prevent this test from being run. This is the
	// only scenario where we prefer the OS heuristic over doing the actual work of
	// validating filesystem case sensitivity via `fs.IsCaseSensitiveFilesystem`.
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("skip this test on non-Windows, non-macOS")
	}

	h := test.NewHelper(t)
	defer h.Cleanup()

	h.TempDir("")
	tmpPath := h.Path(".")

	// First, check the scenarios for which we expect an error.
	_, err := ReadActualFilenames(filepath.Join(tmpPath, "does_not_exists"), []string{""})
	switch {
	case err == nil:
		t.Fatal("expected err for non-existing folder")
	// use `errors.Cause` because the error is wrapped and returned
	case !os.IsNotExist(errors.Cause(err)):
		t.Fatalf("unexpected error: %+v", err)
	}
	h.TempFile("tmpFile", "")
	_, err = ReadActualFilenames(h.Path("tmpFile"), []string{""})
	switch {
	case err == nil:
		t.Fatal("expected err for passing file instead of directory")
	case err != errPathNotDir:
		t.Fatalf("unexpected error: %+v", err)
	}

	cases := []struct {
		createFiles []string
		names       []string
		want        map[string]string
	}{
		// If we supply no filenames to the function, it should return an empty map.
		{nil, nil, map[string]string{}},
		// If the directory contains the given file with different case, it should return
		// a map which has the given filename as the key and actual filename as the value.
		{
			[]string{"test1.txt"},
			[]string{"Test1.txt"},
			map[string]string{"Test1.txt": "test1.txt"},
		},
		// 1. If the given filename is same as the actual filename, map should have the
		//    same key and value for the file.
		// 2. If the given filename is present with different case for file extension,
		//    it should return a map which has the given filename as the key and actual
		//    filename as the value.
		// 3. If the given filename is not present even with a different case, the map
		//    returned should not have an entry for that filename.
		{
			[]string{"test2.txt", "test3.TXT"},
			[]string{"test2.txt", "Test3.txt", "Test4.txt"},
			map[string]string{
				"test2.txt": "test2.txt",
				"Test3.txt": "test3.TXT",
			},
		},
	}
	for _, c := range cases {
		for _, file := range c.createFiles {
			h.TempFile(file, "")
		}
		got, err := ReadActualFilenames(tmpPath, c.names)
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if !reflect.DeepEqual(c.want, got) {
			t.Fatalf("returned value does not match expected: \n\t(GOT) %v\n\t(WNT) %v",
				got, c.want)
		}
	}
}

func TestGenTestFilename(t *testing.T) {
	cases := []struct {
		str  string
		want string
	}{
		{"abc", "Abc"},
		{"ABC", "aBC"},
		{"AbC", "abC"},
		{"αβγ", "Αβγ"},
		{"123", "123"},
		{"1a2", "1A2"},
		{"12a", "12A"},
		{"⌘", "⌘"},
	}

	for _, c := range cases {
		got := genTestFilename(c.str)
		if c.want != got {
			t.Fatalf("str: %q, expected: %q, got: %q", c.str, c.want, got)
		}
	}
}

func BenchmarkGenTestFilename(b *testing.B) {
	cases := []string{
		strings.Repeat("a", 128),
		strings.Repeat("A", 128),
		strings.Repeat("α", 128),
		strings.Repeat("1", 128),
		strings.Repeat("⌘", 128),
	}

	for i := 0; i < b.N; i++ {
		for _, str := range cases {
			genTestFilename(str)
		}
	}
}

func TestCopyDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "dep")
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
		// Microsoft Windows. Skipping this this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	var srcdir, dstdir string

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		srcdir = filepath.Join(dir, "src")
		return os.MkdirAll(srcdir, 0755)
	})
	defer cleanup()

	dir, err := ioutil.TempDir("", "dep")
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
		// Microsoft Windows. Skipping this this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	var srcdir, dstdir string

	dir, err := ioutil.TempDir("", "dep")
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

	dir, err := ioutil.TempDir("", "dep")
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

	dir, err := ioutil.TempDir("", "dep")
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
		// this this until a compatible implementation is
		// provided.
		t.Skip("skipping on windows")
	}

	var srcdir, dstdir string

	dir, err := ioutil.TempDir("", "dep")
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
	dir, err := ioutil.TempDir("", "dep")
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

func TestCopyFileSymlink(t *testing.T) {
	h := test.NewHelper(t)
	defer h.Cleanup()
	h.TempDir(".")

	testcases := map[string]string{
		filepath.Join("./testdata/symlinks/file-symlink"):         filepath.Join(h.Path("."), "dst-file"),
		filepath.Join("./testdata/symlinks/windows-file-symlink"): filepath.Join(h.Path("."), "windows-dst-file"),
		filepath.Join("./testdata/symlinks/invalid-symlink"):      filepath.Join(h.Path("."), "invalid-symlink"),
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
				h.Must(err)
				dstb, err := ioutil.ReadFile(dst)
				h.Must(err)

				want = string(srcb)
				got = string(dstb)
			} else {
				want, err = os.Readlink(symlink)
				h.Must(err)

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

func TestCopyFileLongFilePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		// We want to ensure the temporary fix actually fixes the issue with
		// os.Chmod and long file paths. This is only applicable on Windows.
		t.Skip("skipping on non-windows")
	}

	h := test.NewHelper(t)
	h.TempDir(".")
	defer h.Cleanup()

	tmpPath := h.Path(".")

	// Create a directory with a long-enough path name to cause the bug in #774.
	dirName := ""
	for len(tmpPath+string(os.PathSeparator)+dirName) <= 300 {
		dirName += "directory"
	}

	h.TempDir(dirName)
	h.TempFile(dirName+string(os.PathSeparator)+"src", "")

	tmpDirPath := tmpPath + string(os.PathSeparator) + dirName + string(os.PathSeparator)

	err := copyFile(tmpDirPath+"src", tmpDirPath+"dst")
	if err != nil {
		t.Fatalf("unexpected error while copying file: %v", err)
	}
}

// C:\Users\appveyor\AppData\Local\Temp\1\gotest639065787\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890\dir4567890

func TestCopyFileFail(t *testing.T) {
	if runtime.GOOS == "windows" {
		// XXX: setting permissions works differently in
		// Microsoft Windows. Skipping this this until a
		// compatible implementation is provided.
		t.Skip("skipping on windows")
	}

	dir, err := ioutil.TempDir("", "dep")
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
// directory in it, in such a way that that directory is not accessible
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
	dir, err := ioutil.TempDir("", "dep")
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

func TestEnsureDir(t *testing.T) {
	h := test.NewHelper(t)
	defer h.Cleanup()
	h.TempDir(".")
	h.TempFile("file", "")

	tmpPath := h.Path(".")

	var dn string
	cleanup := setupInaccessibleDir(t, func(dir string) error {
		dn = filepath.Join(dir, "dir")
		return os.Mkdir(dn, 0777)
	})
	defer cleanup()

	tests := map[string]bool{
		// [success] A dir already exists for the given path.
		tmpPath: true,
		// [success] Dir does not exist but parent dir exists, so should get created.
		filepath.Join(tmpPath, "testdir"): true,
		// [failure] Dir and parent dir do not exist, should return an error.
		filepath.Join(tmpPath, "notexist", "testdir"): false,
		// [failure] Regular file present at given path.
		h.Path("file"): false,
		// [failure] Path inaccessible.
		dn: false,
	}

	if runtime.GOOS == "windows" {
		// This test doesn't work on Microsoft Windows because
		// of the differences in how file permissions are
		// implemented. For this to work, the directory where
		// the directory exists should be inaccessible.
		delete(tests, dn)
	}

	for path, shouldEnsure := range tests {
		err := EnsureDir(path, 0777)
		if shouldEnsure {
			if err != nil {
				t.Fatalf("unexpected error %q for %q", err, path)
			} else if ok, err := IsDir(path); !ok {
				t.Fatalf("expected directory to be preset at %q", path)
				t.Fatal(err)
			}
		} else if err == nil {
			t.Fatalf("expected error for path %q, got none", path)
		}
	}
}

func TestIsRegular(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	var fn string

	cleanup := setupInaccessibleDir(t, func(dir string) error {
		fn = filepath.Join(dir, "file")
		fh, err := os.Create(fn)
		if err != nil {
			return err
		}

		return fh.Close()
	})
	defer cleanup()

	tests := map[string]struct {
		exists bool
		err    bool
	}{
		wd: {false, true},
		filepath.Join(wd, "testdata"):                       {false, true},
		filepath.Join(wd, "testdata", "test.file"):          {true, false},
		filepath.Join(wd, "this_file_does_not_exist.thing"): {false, false},
		fn: {false, true},
	}

	if runtime.GOOS == "windows" {
		// This test doesn't work on Microsoft Windows because
		// of the differences in how file permissions are
		// implemented. For this to work, the directory where
		// the file exists should be inaccessible.
		delete(tests, fn)
	}

	for f, want := range tests {
		got, err := IsRegular(f)
		if err != nil {
			if want.exists != got {
				t.Fatalf("expected %t for %s, got %t", want.exists, f, got)
			}
			if !want.err {
				t.Fatalf("expected no error, got %v", err)
			}
		} else {
			if want.err {
				t.Fatalf("expected error for %s, got none", f)
			}
		}

		if got != want.exists {
			t.Fatalf("expected %t for %s, got %t", want, f, got)
		}
	}

}

func TestIsDir(t *testing.T) {
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
		wd: {true, false},
		filepath.Join(wd, "testdata"):                       {true, false},
		filepath.Join(wd, "main.go"):                        {false, true},
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

func TestIsNonEmptyDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	h := test.NewHelper(t)
	defer h.Cleanup()

	h.TempDir("empty")

	testCases := []struct {
		path  string
		empty bool
		err   bool
	}{
		{wd, true, false},
		{"testdata", true, false},
		{filepath.Join(wd, "fs.go"), false, true},
		{filepath.Join(wd, "this_file_does_not_exist.thing"), false, false},
		{h.Path("empty"), false, false},
	}

	// This test case doesn't work on Microsoft Windows because of the
	// differences in how file permissions are implemented.
	if runtime.GOOS != "windows" {
		var inaccessibleDir string
		cleanup := setupInaccessibleDir(t, func(dir string) error {
			inaccessibleDir = filepath.Join(dir, "empty")
			return os.Mkdir(inaccessibleDir, 0777)
		})
		defer cleanup()

		testCases = append(testCases, struct {
			path  string
			empty bool
			err   bool
		}{inaccessibleDir, false, true})
	}

	for _, want := range testCases {
		got, err := IsNonEmptyDir(want.path)
		if want.err && err == nil {
			if got {
				t.Fatalf("wanted false with error for %v, but got true", want.path)
			}
			t.Fatalf("wanted an error for %v, but it was nil", want.path)
		}

		if got != want.empty {
			t.Fatalf("wanted %t for %v, but got %t", want.empty, want.path, got)
		}
	}
}

func TestIsSymlink(t *testing.T) {
	dir, err := ioutil.TempDir("", "dep")
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
