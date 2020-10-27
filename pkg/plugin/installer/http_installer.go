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

package installer // import "helm.sh/helm/v3/pkg/plugin/installer"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/third_party/dep/fs"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/plugin/cache"
)

// HTTPInstaller installs plugins from an archive served by a web server.
type HTTPInstaller struct {
	CacheDir   string
	PluginName string
	base
	extractor Extractor
	getter    getter.Getter
}

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
// This should be refactored in Helm 4, combined with the extension-based mechanism.
func mediaTypeToExtension(mt string) (string, bool) {
	switch strings.ToLower(mt) {
	case "application/gzip", "application/x-gzip", "application/x-tgz", "application/x-gtar":
		return ".tgz", true
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
	return nil, errors.Errorf("no extractor implemented yet for %s", source)
}

// NewHTTPInstaller creates a new HttpInstaller.
func NewHTTPInstaller(source string) (*HTTPInstaller, error) {
	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}

	extractor, err := NewExtractor(source)
	if err != nil {
		return nil, err
	}

	get, err := getter.All(new(cli.EnvSettings)).ByScheme("http")
	if err != nil {
		return nil, err
	}

	i := &HTTPInstaller{
		CacheDir:   helmpath.CachePath("plugins", key),
		PluginName: stripPluginName(filepath.Base(source)),
		base:       newBase(source),
		extractor:  extractor,
		getter:     get,
	}
	return i, nil
}

// helper that relies on some sort of convention for plugin name (plugin-name-<version>)
func stripPluginName(name string) string {
	var strippedName string
	for suffix := range Extractors {
		if strings.HasSuffix(name, suffix) {
			strippedName = strings.TrimSuffix(name, suffix)
			break
		}
	}
	re := regexp.MustCompile(`(.*)-[0-9]+\..*`)
	return re.ReplaceAllString(strippedName, `$1`)
}

// Install downloads and extracts the tarball into the cache directory
// and installs into the plugin directory.
//
// Implements Installer.
func (i *HTTPInstaller) Install() error {
	pluginData, err := i.getter.Get(i.Source)
	if err != nil {
		return err
	}

	if err := i.extractor.Extract(pluginData, i.CacheDir); err != nil {
		return errors.Wrap(err, "extracting files from archive")
	}

	if !isPlugin(i.CacheDir) {
		return ErrMissingMetadata
	}

	src, err := filepath.Abs(i.CacheDir)
	if err != nil {
		return err
	}

	debug("copying %s to %s", src, i.Path())
	return fs.CopyDir(src, i.Path())
}

// Update updates a local repository
// Not implemented for now since tarball most likely will be packaged by version
func (i *HTTPInstaller) Update() error {
	return errors.Errorf("method Update() not implemented for HttpInstaller")
}

// Path is overridden because we want to join on the plugin name not the file name
func (i HTTPInstaller) Path() string {
	if i.base.Source == "" {
		return ""
	}
	return helmpath.DataPath("plugins", i.PluginName)
}

// CleanJoin resolves dest as a subpath of root.
//
// This function runs several security checks on the path, generating an error if
// the supplied `dest` looks suspicious or would result in dubious behavior on the
// filesystem.
//
// CleanJoin assumes that any attempt by `dest` to break out of the CWD is an attempt
// to be malicious. (If you don't care about this, use the securejoin-filepath library.)
// It will emit an error if it detects paths that _look_ malicious, operating on the
// assumption that we don't actually want to do anything with files that already
// appear to be nefarious.
//
//   - The character `:` is considered illegal because it is a separator on UNIX and a
//     drive designator on Windows.
//   - The path component `..` is considered suspicions, and therefore illegal
//   - The character \ (backslash) is treated as a path separator and is converted to /.
//   - Beginning a path with a path separator is illegal
//   - Rudimentary symlink protects are offered by SecureJoin.
func cleanJoin(root, dest string) (string, error) {

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
	for _, part := range strings.Split(dest, "/") {
		if part == ".." {
			return "", errors.New("path contains '..', which is illegal")
		}
	}

	// If a path is absolute, the creator of the TAR is doing something shady.
	if path.IsAbs(dest) {
		return "", errors.New("path is absolute, which is illegal")
	}

	// SecureJoin will do some cleaning, as well as some rudimentary checking of symlinks.
	newpath, err := securejoin.SecureJoin(root, dest)
	if err != nil {
		return "", err
	}

	return filepath.ToSlash(newpath), nil
}

// Extract extracts compressed archives
//
// Implements Extractor.
func (g *TarGzExtractor) Extract(buffer *bytes.Buffer, targetDir string) error {
	uncompressedStream, err := gzip.NewReader(buffer)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path, err := cleanJoin(targetDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
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
			return errors.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}
	return nil
}
