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
	"bytes" // TarGzExtractor extracts gzip compressed tar archives
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	fp "github.com/cyphar/filepath-securejoin"
)

// Extractor provides an interface for extracting archives
type Extractor interface {
	Extract(buffer *bytes.Buffer, targetDir string) (string, error)
}

// TarGzExtractor extracts GZip compressed tar archives
type TarGzExtractor struct {
	extension string
}

// Extractors contains a map of suffixes and matching implementations of extractor to return
var extractors = map[string]Extractor{
	".tar.gz": &TarGzExtractor{},
	".tgz":    &TarGzExtractor{},
}

// NewExtractor creates a new extractor matching the source file name
func NewExtractor(source string) (Extractor, error) {
	for suffix, extractor := range extractors {
		if strings.HasSuffix(source, suffix) {
			return extractor, nil
		}
	}
	return nil, fmt.Errorf("no extractor implemented yet for %s", source)
}

// StripPluginName relies on some sort of convention for plugin name (plugin-name-<version>)
func stripPluginName(name string) string {
	var strippedName string
	for suffix := range extractors {
		if strings.HasSuffix(name, suffix) {
			strippedName = strings.TrimSuffix(name, suffix)
			break
		}
	}
	re := regexp.MustCompile(`(.*)-[0-9]+\..*`)
	return re.ReplaceAllString(strippedName, `$1`)
}

// Extract extracts compressed archives
//
// Implements Extractor. Returns the directory where the plugin.yaml is located or an error
func (g *TarGzExtractor) Extract(buffer *bytes.Buffer, targetDir string) (string, error) {
	uncompressedStream, err := gzip.NewReader(buffer)
	if err != nil {
		return "", err
	}
	defer uncompressedStream.Close()

	tarReader := tar.NewReader(uncompressedStream)

	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return "", err
	}

	pluginDir := targetDir

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return "", err
		}

		path, err := fp.SecureJoin(targetDir, header.Name)
		if err != nil {
			return "", err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close() // Manually close since defering in a loop may cause a resource leak

			path, file := filepath.Split(outFile.Name())
			if file == pluginFile {
				pluginDir = path
			}
		default:
			return "", fmt.Errorf("unknown type: %b in %s", header.Typeflag, header.Name)
		}
	}

	return pluginDir, nil
}
