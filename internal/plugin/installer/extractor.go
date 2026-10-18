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

package installer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// TarGzExtractor extracts gzip compressed tar archives
type TarGzExtractor struct{}

// Extractor provides an interface for extracting archives
type Extractor interface {
	Extract(buffer *bytes.Buffer, targetDir string) error
}

// Extractors contains a map of suffixes and matching implementations of extractor to return
var Extractors = map[string]Extractor{
	".tar.gz": &TarGzExtractor{},
	".tgz":    &TarGzExtractor{},
}

// Convert a media type to an extractor extension.
//
// TODO Helm v5: refactor this, combined with the extension-based mechanism.
func mediaTypeToExtension(mt string) (string, bool) {
	switch strings.ToLower(mt) {
	case "application/gzip", "application/x-gzip", "application/x-tgz", "application/x-gtar":
		return ".tgz", true
	case "application/octet-stream":
		// Generic binary type - we'll need to check the URL suffix
		return "", false
	default:
		return "", false
	}
}

// NewExtractor creates a new extractor matching the source file name
func NewExtractor(source string) (Extractor, error) {
	for suffix, extractor := range Extractors {
		if strings.HasSuffix(source, suffix) {
			return extractor, nil
		}
	}
	return nil, fmt.Errorf("no extractor implemented yet for %s", source)
}

// cleanPath validates dest, a path read from an archive, and returns it as a
// clean relative path using / as the separator.
//
// This function runs several security checks on the path, generating an error if
// the supplied `dest` looks suspicious or would result in dubious behavior on the
// filesystem.
//
// cleanPath assumes that any attempt by `dest` to break out of the CWD is an attempt
// to be malicious. It will emit an error if it detects paths that _look_ malicious,
// operating on the assumption that we don't actually want to do anything with files
// that already appear to be nefarious.
//
//   - The character `:` is considered illegal because it is a separator on UNIX and a
//     drive designator on Windows.
//   - The path component `..` is considered suspicious, and therefore illegal
//   - The character \ (backslash) is treated as a path separator and is converted to /.
//   - Beginning a path with a path separator is illegal
//
// Symlinks are not handled here: extractTar opens every path through an os.Root.
func cleanPath(dest string) (string, error) {
	// On Windows, this is a drive separator. On UNIX-like, this is the path list separator.
	// In neither case do we want to trust a TAR that contains these.
	if strings.Contains(dest, ":") {
		return "", errors.New("path contains ':', which is illegal")
	}

	// The Go tar library does not convert separators for us.
	// We assume here, as we do elsewhere, that `\\` means a Windows path.
	dest = strings.ReplaceAll(dest, "\\", "/")

	// We want to alert the user that something bad was attempted. Cleaning it
	// is not a good practice.
	if slices.Contains(strings.Split(dest, "/"), "..") {
		return "", errors.New("path contains '..', which is illegal")
	}

	// If a path is absolute, the creator of the TAR is doing something shady.
	if path.IsAbs(dest) {
		return "", errors.New("path is absolute, which is illegal")
	}

	return path.Clean(dest), nil
}

// Extract extracts compressed archives
//
// Implements Extractor.
func (g *TarGzExtractor) Extract(buffer *bytes.Buffer, targetDir string) error {
	uncompressedStream, err := gzip.NewReader(buffer)
	if err != nil {
		return err
	}

	return extractTar(uncompressedStream, targetDir)
}

// extractTar extracts a tar archive into targetDir, creating it if needed.
//
// Every file and directory is created through an os.Root opened on targetDir,
// so an entry can never write outside of it, even when targetDir already
// contains symlinks or is modified while the archive is being extracted.
func extractTar(r io.Reader, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(targetDir)
	if err != nil {
		return err
	}
	defer root.Close()

	tarReader := tar.NewReader(r)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		name, err := cleanPath(header.Name)
		if err != nil {
			return err
		}
		name = filepath.FromSlash(name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				return err
			}
			outFile, err := root.OpenFile(name, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode).Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		// We don't want to process these extension header files.
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		default:
			return fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}
	return nil
}

// stripPluginName is a helper that relies on some sort of convention for plugin name (plugin-name-<version>)
func stripPluginName(name string) string {
	var strippedName string
	for suffix := range Extractors {
		if before, ok := strings.CutSuffix(name, suffix); ok {
			strippedName = before
			break
		}
	}
	re := regexp.MustCompile(`(.*)-\d+\..*`)
	return re.ReplaceAllString(strippedName, `$1`)
}
