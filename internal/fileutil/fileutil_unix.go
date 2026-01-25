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

package fileutil

import (
	"io"
	"os"
)

// LockedAtomicWriteFile atomically writes a file to disk.
//
// On non-Windows platforms, this is a simple wrapper around AtomicWriteFile
// since concurrent file access errors are not an issue on Unix-like systems.
//
// Returns true if the file was written, false if it already existed.
func LockedAtomicWriteFile(filename string, reader io.Reader, mode os.FileMode) (bool, error) {
	// Check if the file already exists
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
