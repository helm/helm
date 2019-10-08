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

package loader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chart"
)

var drivePathPattern = regexp.MustCompile(`^[a-zA-Z]:/`)

// FileLoader loads a chart from a file
type FileLoader string

// Load loads a chart
func (l FileLoader) Load() (*chart.Chart, error) {
	return LoadFile(string(l))
}

// LoadFile loads from an archive file.
func LoadFile(name string) (*chart.Chart, error) {
	if fi, err := os.Stat(name); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("cannot load a directory")
	}

	raw, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer raw.Close()

	return LoadArchive(raw)
}

// LoadArchiveFiles reads in files out of an archive into memory. This function
// performs important path security checks and should always be used before
// expanding a tarball
func LoadArchiveFiles(in io.Reader) ([]*BufferedFile, error) {
	unzipped, err := gzip.NewReader(in)
	if err != nil {
		return nil, err
	}
	defer unzipped.Close()

	files := []*BufferedFile{}
	tr := tar.NewReader(unzipped)
	for {
		b := bytes.NewBuffer(nil)
		hd, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hd.FileInfo().IsDir() {
			// Use this instead of hd.Typeflag because we don't have to do any
			// inference chasing.
			continue
		}

		// Archive could contain \ if generated on Windows
		delimiter := "/"
		if strings.ContainsRune(hd.Name, '\\') {
			delimiter = "\\"
		}

		parts := strings.Split(hd.Name, delimiter)
		n := strings.Join(parts[1:], delimiter)

		// Normalize the path to the / delimiter
		n = strings.ReplaceAll(n, delimiter, "/")

		if path.IsAbs(n) {
			return nil, errors.New("chart illegally contains absolute paths")
		}

		n = path.Clean(n)
		if n == "." {
			// In this case, the original path was relative when it should have been absolute.
			return nil, errors.Errorf("chart illegally contains content outside the base directory: %q", hd.Name)
		}
		if strings.HasPrefix(n, "..") {
			return nil, errors.New("chart illegally references parent directory")
		}

		// In some particularly arcane acts of path creativity, it is possible to intermix
		// UNIX and Windows style paths in such a way that you produce a result of the form
		// c:/foo even after all the built-in absolute path checks. So we explicitly check
		// for this condition.
		if drivePathPattern.MatchString(n) {
			return nil, errors.New("chart contains illegally named files")
		}

		if parts[0] == "Chart.yaml" {
			return nil, errors.New("chart yaml not in base directory")
		}

		if _, err := io.Copy(b, tr); err != nil {
			return nil, err
		}

		files = append(files, &BufferedFile{Name: n, Data: b.Bytes()})
		b.Reset()
	}

	if len(files) == 0 {
		return nil, errors.New("no files in chart archive")
	}
	return files, nil
}

// LoadArchive loads from a reader containing a compressed tar archive.
func LoadArchive(in io.Reader) (*chart.Chart, error) {
	files, err := LoadArchiveFiles(in)
	if err != nil {
		return nil, err
	}

	return LoadFiles(files)
}
