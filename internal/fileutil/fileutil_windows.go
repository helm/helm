//go:build windows

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

	"github.com/gofrs/flock"
)

// PlatformAtomicWriteFile atomically writes a file to disk with file locking to
// prevent concurrent writes. This is particularly useful on Windows where
// concurrent writes to the same file can cause "Access Denied" errors.
//
// The function acquires a lock on the target file and performs an atomic write,
// preserving the existing behaviour of overwriting any previous content once
// the lock is obtained.
func PlatformAtomicWriteFile(filename string, reader io.Reader, mode os.FileMode) error {
	// Use a separate lock file to coordinate access between processes
	// We cannot lock the target file directly as it would prevent the atomic rename
	lockFileName := filename + ".lock"
	fileLock := flock.New(lockFileName)

	// Lock() ensures serialized access - if another process is writing, this will wait
	if err := fileLock.Lock(); err != nil {
		return err
	}
	defer func() {
		fileLock.Unlock()
		// Clean up the lock file
		// Ignore errors as the file might not exist or be in use by another process
		os.Remove(lockFileName)
	}()

	// Perform the atomic write while holding the lock
	return AtomicWriteFile(filename, reader, mode)
}
