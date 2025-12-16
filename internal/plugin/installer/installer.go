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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v4/internal/plugin"
	"helm.sh/helm/v4/pkg/registry"
)

// ErrMissingMetadata indicates that plugin.yaml is missing.
var ErrMissingMetadata = errors.New("plugin metadata (plugin.yaml) missing")

// Debug enables verbose output.
var Debug bool

// Options contains options for plugin installation.
type Options struct {
	// Verify enables signature verification before installation
	Verify bool
	// Keyring is the path to the keyring for verification
	Keyring string
}

// Installer provides an interface for installing helm client plugins.
type Installer interface {
	// Install adds a plugin.
	Install() error
	// Path is the directory of the installed plugin.
	Path() string
	// Update updates a plugin.
	Update() error
}

// Verifier provides an interface for installers that support verification.
type Verifier interface {
	// SupportsVerification returns true if this installer can verify plugins
	SupportsVerification() bool
	// GetVerificationData returns plugin and provenance data for verification
	GetVerificationData() (archiveData, provData []byte, filename string, err error)
}

// Install installs a plugin.
func Install(i Installer) error {
	_, err := InstallWithOptions(i, Options{})
	return err
}

// VerificationResult contains the result of plugin verification
type VerificationResult struct {
	SignedBy    []string
	Fingerprint string
	FileHash    string
}

// InstallWithOptions installs a plugin with options.
func InstallWithOptions(i Installer, opts Options) (*VerificationResult, error) {

	if err := os.MkdirAll(filepath.Dir(i.Path()), 0755); err != nil {
		return nil, err
	}
	if _, pathErr := os.Stat(i.Path()); !os.IsNotExist(pathErr) {
		slog.Warn("plugin already exists", "path", i.Path(), slog.Any("error", pathErr))
		return nil, errors.New("plugin already exists")
	}

	var result *VerificationResult

	// If verification is requested, check if installer supports it
	if opts.Verify {
		verifier, ok := i.(Verifier)
		if !ok || !verifier.SupportsVerification() {
			return nil, fmt.Errorf("--verify is only supported for plugin tarballs (.tgz files)")
		}

		// Get verification data (works for both memory and file-based installers)
		archiveData, provData, filename, err := verifier.GetVerificationData()
		if err != nil {
			return nil, fmt.Errorf("failed to get verification data: %w", err)
		}

		// Check if provenance data exists
		if len(provData) == 0 {
			// No .prov file found - emit warning but continue installation
			fmt.Fprintf(os.Stderr, "WARNING: No provenance file found for plugin. Plugin is not signed and cannot be verified.\n")
		} else {
			// Provenance data exists - verify the plugin
			verification, err := plugin.VerifyPlugin(archiveData, provData, filename, opts.Keyring)
			if err != nil {
				return nil, fmt.Errorf("plugin verification failed: %w", err)
			}

			// Collect verification info
			result = &VerificationResult{
				SignedBy:    make([]string, 0),
				Fingerprint: fmt.Sprintf("%X", verification.SignedBy.PrimaryKey.Fingerprint),
				FileHash:    verification.FileHash,
			}
			for name := range verification.SignedBy.Identities {
				result.SignedBy = append(result.SignedBy, name)
			}
		}
	}

	if err := i.Install(); err != nil {
		return nil, err
	}

	return result, nil
}

// Update updates a plugin.
func Update(i Installer) error {
	if _, pathErr := os.Stat(i.Path()); os.IsNotExist(pathErr) {
		slog.Warn("plugin does not exist", "path", i.Path(), slog.Any("error", pathErr))
		return errors.New("plugin does not exist")
	}
	return i.Update()
}

// NewForSource determines the correct Installer for the given source.
func NewForSource(source, version string) (installer Installer, err error) {
	if strings.HasPrefix(source, fmt.Sprintf("%s://", registry.OCIScheme)) {
		// Source is an OCI registry reference
		installer, err = NewOCIInstaller(source)
	} else if isLocalReference(source) {
		// Source is a local directory
		installer, err = NewLocalInstaller(source)
	} else if isRemoteHTTPArchive(source) {
		installer, err = NewHTTPInstaller(source)
	} else {
		installer, err = NewVCSInstaller(source, version)
	}

	if err != nil {
		return installer, fmt.Errorf("cannot get information about plugin source %q (if it's a local directory, does it exist?), last error was: %w", source, err)
	}

	return
}

// FindSource determines the correct Installer for the given source.
func FindSource(location string, version string) (Installer, error) {
	installer, err := existingVCSRepo(location, version)
	if err != nil && err.Error() == "Cannot detect VCS" {
		slog.Warn("cannot get information about plugin source", "location", location, slog.Any("error", err))
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
		// First, check if the URL ends with a known archive suffix
		// This is more reliable than content-type detection
		for suffix := range Extractors {
			if strings.HasSuffix(source, suffix) {
				return true
			}
		}

		// If no suffix match, try HEAD request to check content type
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
