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

package fileutil

import (
	"io"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"helm.sh/helm/v4/internal/third_party/dep/fs"
)

// AtomicWriteFile atomically (as atomic as os.Rename allows) writes a file to a
// disk.
func AtomicWriteFile(filename string, reader io.Reader, mode os.FileMode) error {
	tempFile, err := os.CreateTemp(filepath.Split(filename))
	if err != nil {
		return err
	}
	tempName := tempFile.Name()

	if _, err := io.Copy(tempFile, reader); err != nil {
		tempFile.Close() // return value is ignored as we are already on error path
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tempName, mode); err != nil {
		return err
	}

	return fs.RenameWithFallback(tempName, filename)
}

// LockedAtomicWriteFile atomically writes a file to disk with file locking to prevent
// concurrent writes. This is particularly useful on Windows where concurrent writes
// to the same file can cause "Access Denied" errors.
//
// The function acquires a lock on the target file, checks if it already exists,
// and only writes if it doesn't exist. This prevents duplicate work when multiple
// processes try to write the same file simultaneously.
//
// Returns true if the file was written, false if it already existed.
func LockedAtomicWriteFile(filename string, reader io.Reader, mode os.FileMode) (bool, error) {
	// Acquire a file lock for this specific file path to prevent concurrent writes
	fileLock := flock.New(filename + ".lock")
	if err := fileLock.Lock(); err != nil {
		return false, err
	}
	defer fileLock.Unlock()

	// Check if the file already exists (another process might have already written it)
	if _, err := os.Stat(filename); err == nil {
		// File already exists, skip writing
		return false, nil
	}

	// File doesn't exist, write it atomically
	if err := AtomicWriteFile(filename, reader, mode); err != nil {
		return false, err
	}

	return true, nil
}
