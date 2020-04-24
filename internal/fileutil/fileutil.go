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
	"io/ioutil"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/internal/third_party/dep/fs"
)

// AtomicWriteFile atomically (as atomic as os.Rename allows) writes a file to a
// disk.
func AtomicWriteFile(filename string, reader io.Reader, mode os.FileMode) error {
	tempFile, err := ioutil.TempFile(filepath.Split(filename))
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
