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
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/chart"
)

// MaxDecompressedChartSize is the maximum size of a chart archive that will be
// decompressed. This is the decompressed size of all the files.
// The default value is 100 MiB.
var MaxDecompressedChartSize int64 = 100 * 1024 * 1024 // Default 100 MiB

// MaxDecompressedFileSize is the size of the largest file that Helm will attempt to load.
// The size of the file is the decompressed version of it when it is stored in an archive.
var MaxDecompressedFileSize int64 = 5 * 1024 * 1024 // Default 5 MiB

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

	err = ensureArchive(name, raw)
	if err != nil {
		return nil, err
	}

	c, err := LoadArchive(raw)
	if err != nil {
		if err == gzip.ErrHeader {
			return nil, fmt.Errorf("file '%s' does not appear to be a valid chart file (details: %s)", name, err)
		}
	}
	return c, err
}

// ensureArchive's job is to return an informative error if the file does not appear to be a gzipped archive.
//
// Sometimes users will provide a values.yaml for an argument where a chart is expected. One common occurrence
// of this is invoking `helm template values.yaml mychart` which would otherwise produce a confusing error
// if we didn't check for this.
func ensureArchive(name string, raw *os.File) error {
	defer raw.Seek(0, 0) // reset read offset to allow archive loading to proceed.

	// Check the file format to give us a chance to provide the user with more actionable feedback.
	buffer := make([]byte, 512)
	_, err := raw.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("file '%s' cannot be read: %s", name, err)
	}

	// Helm may identify achieve of the application/x-gzip as application/vnd.ms-fontobject.
	// Fix for: https://github.com/helm/helm/issues/12261
	if contentType := http.DetectContentType(buffer); contentType != "application/x-gzip" && !isGZipApplication(buffer) {
		// TODO: Is there a way to reliably test if a file content is YAML? ghodss/yaml accepts a wide
		//       variety of content (Makefile, .zshrc) as valid YAML without errors.

		// Wrong content type. Let's check if it's yaml and give an extra hint?
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			return fmt.Errorf("file '%s' seems to be a YAML file, but expected a gzipped archive", name)
		}
		return fmt.Errorf("file '%s' does not appear to be a gzipped archive; got '%s'", name, contentType)
	}
	return nil
}

// isGZipApplication checks whether the archive is of the application/x-gzip type.
func isGZipApplication(data []byte) bool {
	sig := []byte("\x1F\x8B\x08")
	return bytes.HasPrefix(data, sig)
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
	remainingSize := MaxDecompressedChartSize
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

		switch hd.Typeflag {
		// We don't want to process these extension header files.
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
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

		if hd.Size > remainingSize {
			return nil, fmt.Errorf("decompressed chart is larger than the maximum file size %d", MaxDecompressedChartSize)
		}

		if hd.Size > MaxDecompressedFileSize {
			return nil, fmt.Errorf("decompressed chart file %q is larger than the maximum file size %d", hd.Name, MaxDecompressedFileSize)
		}

		limitedReader := io.LimitReader(tr, remainingSize)

		bytesWritten, err := io.Copy(b, limitedReader)
		if err != nil {
			return nil, err
		}

		remainingSize -= bytesWritten
		// When the bytesWritten are less than the file size it means the limit reader ended
		// copying early. Here we report that error. This is important if the last file extracted
		// is the one that goes over the limit. It assumes the Size stored in the tar header
		// is correct, something many applications do.
		if bytesWritten < hd.Size || remainingSize <= 0 {
			return nil, fmt.Errorf("decompressed chart is larger than the maximum file size %d", MaxDecompressedChartSize)
		}

		data := bytes.TrimPrefix(b.Bytes(), utf8bom)

		files = append(files, &BufferedFile{Name: n, Data: data})
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
