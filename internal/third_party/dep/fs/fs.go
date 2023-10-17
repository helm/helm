/*
Copyright (c) for portions of fs.go are held by The Go Authors, 2016 and are provided under
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
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/pkg/errors"
)

// fs contains a copy of a few functions from dep tool code to avoid a dependency on golang/dep.
// This code is copied from https://github.com/golang/dep/blob/37d6c560cdf407be7b6cd035b23dba89df9275cf/internal/fs/fs.go
// No changes to the code were made other than removing some unused functions

// RenameWithFallback attempts to rename a file or directory, but falls back to
// copying in the event of a cross-device link error. If the fallback copy
// succeeds, src is still removed, emulating normal rename behavior.
func RenameWithFallback(src, dst string) error {
	_, err := os.Stat(src)
	if err != nil {
		return errors.Wrapf(err, "cannot stat %s", src)
	}

	err = os.Rename(src, dst)
	if err == nil {
		return nil
	}

	return renameFallback(err, src, dst)
}

// renameByCopy attempts to rename a file or directory by copying it to the
// destination and then removing the src thus emulating the rename behavior.
func renameByCopy(src, dst string) error {
	var cerr error
	if dir, _ := IsDir(src); dir {
		cerr = CopyDir(src, dst)
		if cerr != nil {
			cerr = errors.Wrap(cerr, "copying directory failed")
		}
	} else {
		cerr = copyFile(src, dst)
		if cerr != nil {
			cerr = errors.Wrap(cerr, "copying file failed")
		}
	}

	if cerr != nil {
		return errors.Wrapf(cerr, "rename fallback failed: cannot rename %s to %s", src, dst)
	}

	return errors.Wrapf(os.RemoveAll(src), "cannot delete %s", src)
}

var (
	errSrcNotDir = errors.New("source is not a directory")
	errDstExist  = errors.New("destination already exists")
)

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
func CopyDir(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	// We use os.Lstat() here to ensure we don't fall in a loop where a symlink
	// actually links to a one of its parent directories.
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return errSrcNotDir
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		return errDstExist
	}

	if err = os.MkdirAll(dst, fi.Mode()); err != nil {
		return errors.Wrapf(err, "cannot mkdir %s", dst)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return errors.Wrapf(err, "cannot read directory %s", dst)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err = CopyDir(srcPath, dstPath); err != nil {
				return errors.Wrap(err, "copying directory failed")
			}
		} else {
			// This will include symlinks, which is what we want when
			// copying things.
			if err = copyFile(srcPath, dstPath); err != nil {
				return errors.Wrap(err, "copying file failed")
			}
		}
	}

	return nil
}

// copyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all its contents will be replaced by the contents
// of the source file. The file mode will be copied from the source.
func copyFile(src, dst string) (err error) {
	if sym, err := IsSymlink(src); err != nil {
		return errors.Wrap(err, "symlink check failed")
	} else if sym {
		if err := cloneSymlink(src, dst); err != nil {
			if runtime.GOOS == "windows" {
				// If cloning the symlink fails on Windows because the user
				// does not have the required privileges, ignore the error and
				// fall back to copying the file contents.
				//
				// ERROR_PRIVILEGE_NOT_HELD is 1314 (0x522):
				// https://msdn.microsoft.com/en-us/library/windows/desktop/ms681385(v=vs.85).aspx
				if lerr, ok := err.(*os.LinkError); ok && lerr.Err != syscall.Errno(1314) {
					return err
				}
			} else {
				return err
			}
		} else {
			return nil
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return
	}

	// Check for write errors on Close
	if err = out.Close(); err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}

	// Temporary fix for Go < 1.9
	//
	// See: https://github.com/golang/dep/issues/774
	// and https://github.com/golang/go/issues/20829
	if runtime.GOOS == "windows" {
		dst = fixLongPath(dst)
	}
	err = os.Chmod(dst, si.Mode())

	return
}

// cloneSymlink will create a new symlink that points to the resolved path of sl.
// If sl is a relative symlink, dst will also be a relative symlink.
func cloneSymlink(sl, dst string) error {
	resolved, err := os.Readlink(sl)
	if err != nil {
		return err
	}

	return os.Symlink(resolved, dst)
}

// IsDir determines is the path given is a directory or not.
func IsDir(name string) (bool, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return false, err
	}
	if !fi.IsDir() {
		return false, errors.Errorf("%q is not a directory", name)
	}
	return true, nil
}

// IsSymlink determines if the given path is a symbolic link.
func IsSymlink(path string) (bool, error) {
	l, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	return l.Mode()&os.ModeSymlink == os.ModeSymlink, nil
}

// fixLongPath returns the extended-length (\\?\-prefixed) form of
// path when needed, in order to avoid the default 260 character file
// path limit imposed by Windows. If path is not easily converted to
// the extended-length form (for example, if path is a relative path
// or contains .. elements), or is short enough, fixLongPath returns
// path unmodified.
//
// See https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
func fixLongPath(path string) string {
	// Do nothing (and don't allocate) if the path is "short".
	// Empirically (at least on the Windows Server 2013 builder),
	// the kernel is arbitrarily okay with < 248 bytes. That
	// matches what the docs above say:
	// "When using an API to create a directory, the specified
	// path cannot be so long that you cannot append an 8.3 file
	// name (that is, the directory name cannot exceed MAX_PATH
	// minus 12)." Since MAX_PATH is 260, 260 - 12 = 248.
	//
	// The MSDN docs appear to say that a normal path that is 248 bytes long
	// will work; empirically the path must be less then 248 bytes long.
	if len(path) < 248 {
		// Don't fix. (This is how Go 1.7 and earlier worked,
		// not automatically generating the \\?\ form)
		return path
	}

	// The extended form begins with \\?\, as in
	// \\?\c:\windows\foo.txt or \\?\UNC\server\share\foo.txt.
	// The extended form disables evaluation of . and .. path
	// elements and disables the interpretation of / as equivalent
	// to \. The conversion here rewrites / to \ and elides
	// . elements as well as trailing or duplicate separators. For
	// simplicity it avoids the conversion entirely for relative
	// paths or paths containing .. elements. For now,
	// \\server\share paths are not converted to
	// \\?\UNC\server\share paths because the rules for doing so
	// are less well-specified.
	if len(path) >= 2 && path[:2] == `\\` {
		// Don't canonicalize UNC paths.
		return path
	}
	if !isAbs(path) {
		// Relative path
		return path
	}

	const prefix = `\\?`

	pathbuf := make([]byte, len(prefix)+len(path)+len(`\`))
	copy(pathbuf, prefix)
	n := len(path)
	r, w := 0, len(prefix)
	for r < n {
		switch {
		case os.IsPathSeparator(path[r]):
			// empty block
			r++
		case path[r] == '.' && (r+1 == n || os.IsPathSeparator(path[r+1])):
			// /./
			r++
		case r+1 < n && path[r] == '.' && path[r+1] == '.' && (r+2 == n || os.IsPathSeparator(path[r+2])):
			// /../ is currently unhandled
			return path
		default:
			pathbuf[w] = '\\'
			w++
			for ; r < n && !os.IsPathSeparator(path[r]); r++ {
				pathbuf[w] = path[r]
				w++
			}
		}
	}
	// A drive's root directory needs a trailing \
	if w == len(`\\?\c:`) {
		pathbuf[w] = '\\'
		w++
	}
	return string(pathbuf[:w])
}

func isAbs(path string) (b bool) {
	v := volumeName(path)
	if v == "" {
		return false
	}
	path = path[len(v):]
	if path == "" {
		return false
	}
	return os.IsPathSeparator(path[0])
}

func volumeName(path string) (v string) {
	if len(path) < 2 {
		return ""
	}
	// with drive letter
	c := path[0]
	if path[1] == ':' &&
		('0' <= c && c <= '9' || 'a' <= c && c <= 'z' ||
			'A' <= c && c <= 'Z') {
		return path[:2]
	}
	// is it UNC
	if l := len(path); l >= 5 && os.IsPathSeparator(path[0]) && os.IsPathSeparator(path[1]) &&
		!os.IsPathSeparator(path[2]) && path[2] != '.' {
		// first, leading `\\` and next shouldn't be `\`. its server name.
		for n := 3; n < l-1; n++ {
			// second, next '\' shouldn't be repeated.
			if os.IsPathSeparator(path[n]) {
				n++
				// third, following something characters. its share name.
				if !os.IsPathSeparator(path[n]) {
					if path[n] == '.' {
						break
					}
					for ; n < l; n++ {
						if os.IsPathSeparator(path[n]) {
							break
						}
					}
					return path[:n]
				}
				break
			}
		}
	}
	return ""
}
