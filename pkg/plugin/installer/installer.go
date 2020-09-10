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
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/plugin"
)

// ErrMissingMetadata indicates that plugin.yaml is missing.
var ErrMissingMetadata = errors.New("plugin metadata (plugin.yaml) missing")

// Debug enables verbose output.
var Debug bool

// Installer provides an interface for installing helm client plugins.
type Installer interface {
	// Install adds a plugin.
	Install() error
	// Path is the directory of the installed plugin.
	Path() string
	// Update updates a plugin.
	Update() error
}

// Install installs a plugin.
func Install(i Installer) error {
	if err := os.MkdirAll(filepath.Dir(i.Path()), 0755); err != nil {
		return err
	}
	if _, pathErr := os.Stat(i.Path()); !os.IsNotExist(pathErr) {
		return errors.New("plugin already exists")
	}
	return i.Install()
}

// Update updates a plugin.
func Update(i Installer) error {
	if _, pathErr := os.Stat(i.Path()); os.IsNotExist(pathErr) {
		return errors.New("plugin does not exist")
	}
	return i.Update()
}

// NewForSource determines the correct Installer for the given source.
func NewForSource(source, version string) (Installer, error) {
	// Check if source is a local directory
	if isLocalReference(source) {
		return NewLocalInstaller(source)
	} else if isRemoteHTTPArchive(source) {
		return NewHTTPInstaller(source)
	}
	return NewVCSInstaller(source, version)
}

// FindSource determines the correct Installer for the given source.
func FindSource(location string) (Installer, error) {
	installer, err := existingVCSRepo(location)
	if err != nil && err.Error() == "Cannot detect VCS" {
		return installer, errors.New("cannot get information about plugin source")
	}
	return installer, err
}

// isLocalReference checks if the source exists on the filesystem.
func isLocalReference(source string) bool {
	_, err := os.Stat(source)
	return err == nil
}

// isRemoteHTTPArchive checks if the source is a http/https url and is an archive
//
// It works by checking whether the source looks like a URL and, if it does, running a
// HEAD operation to see if the remote resource is a file that we understand.
func isRemoteHTTPArchive(source string) bool {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		res, err := http.Head(source)
		if err != nil {
			// If we get an error at the network layer, we can't install it. So
			// we return false.
			return false
		}

		// Next, we look for the content type or content disposition headers to see
		// if they have matching extractors.
		contentType := res.Header.Get("content-type")
		foundSuffix, ok := mediaTypeToExtension(contentType)
		if !ok {
			// Media type not recognized
			return false
		}

		for suffix := range Extractors {
			if strings.HasSuffix(foundSuffix, suffix) {
				return true
			}
		}
	}
	return false
}

// isPlugin checks if the directory contains a plugin.yaml file.
func isPlugin(dirname string) bool {
	_, err := os.Stat(filepath.Join(dirname, plugin.PluginFileName))
	return err == nil
}

var logger = log.New(os.Stderr, "[debug] ", log.Lshortfile)

func debug(format string, args ...interface{}) {
	if Debug {
		logger.Output(2, fmt.Sprintf(format, args...))
	}
}
