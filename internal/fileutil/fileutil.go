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
	"strings"

	"archive/tar"
	"bytes"
	"compress/gzip"

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

func CompressDirToTgz(src, tmpdir string) (*bytes.Buffer, error) {
	// tar => gzip => buf

	buf := bytes.NewBuffer(nil)
	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)

	// walk through every file in the folder
	walkErr := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {

		// generate tar header
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(fi, strings.TrimPrefix(file, tmpdir+"/"))
		if err != nil {
			return err
		}

		// must provide real name
		// (see https://golang.org/src/archive/tar/common.go?#L626)
		header.Name = strings.TrimPrefix(filepath.ToSlash(file), tmpdir+"/")

		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// if not a dir, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	// produce tar
	if err := tw.Close(); err != nil {
		return nil, err
	}
	// produce gzip
	if err := zr.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}
