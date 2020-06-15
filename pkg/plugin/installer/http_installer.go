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

package installer // import "k8s.io/helm/pkg/plugin/installer"

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
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"

	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/plugin/cache"
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

// NewExtractor creates a new extractor matching the source file name
func NewExtractor(source string) (Extractor, error) {
	for suffix, extractor := range Extractors {
		if strings.HasSuffix(source, suffix) {
			return extractor, nil
		}
	}
	return nil, fmt.Errorf("no extractor implemented yet for %s", source)
}

// NewHTTPInstaller creates a new HttpInstaller.
func NewHTTPInstaller(source string, home helmpath.Home) (*HTTPInstaller, error) {

	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}

	extractor, err := NewExtractor(source)
	if err != nil {
		return nil, err
	}

	getConstructor, err := getter.ByScheme("http", environment.EnvSettings{})
	if err != nil {
		return nil, err
	}

	get, err := getConstructor.New(source, "", "", "")
	if err != nil {
		return nil, err
	}

	i := &HTTPInstaller{
		CacheDir:   home.Path("cache", "plugins", key),
		PluginName: stripPluginName(filepath.Base(source)),
		base:       newBase(source, home),
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

// Install downloads and extracts the tarball into the cache directory and creates a symlink to the plugin directory in $HELM_HOME.
//
// Implements Installer.
func (i *HTTPInstaller) Install() error {

	pluginData, err := i.getter.Get(i.Source)
	if err != nil {
		return err
	}

	err = i.extractor.Extract(pluginData, i.CacheDir)
	if err != nil {
		return err
	}

	if !isPlugin(i.CacheDir) {
		return ErrMissingMetadata
	}

	src, err := filepath.Abs(i.CacheDir)
	if err != nil {
		return err
	}

	return i.link(src)
}

// Update updates a local repository
// Not implemented for now since tarball most likely will be packaged by version
func (i *HTTPInstaller) Update() error {
	return fmt.Errorf("method Update() not implemented for HttpInstaller")
}

// Override link because we want to use HttpInstaller.Path() not base.Path()
func (i *HTTPInstaller) link(from string) error {
	debug("symlinking %s to %s", from, i.Path())
	return os.Symlink(from, i.Path())
}

// Path is overridden because we want to join on the plugin name not the file name
func (i HTTPInstaller) Path() string {
	if i.base.Source == "" {
		return ""
	}
	return filepath.Join(i.base.HelmHome.Plugins(), i.PluginName)
}

// Extract extracts compressed archives
//
// Implements Extractor.
func (g *TarGzExtractor) Extract(buffer *bytes.Buffer, targetDir string) error {
	uncompressedStream, err := gzip.NewReader(buffer)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(uncompressedStream)

	os.MkdirAll(targetDir, 0755)

	for true {
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
		default:
			return fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}

	return nil

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
