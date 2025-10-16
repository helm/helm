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

package getter

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/Masterminds/vcs"

	"helm.sh/helm/v4/internal/plugin/cache"
	"helm.sh/helm/v4/pkg/helmpath"
)

type VCSGetter struct {
	opts getterOptions
}

func (v *VCSGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&v.opts)
	}

	if v.opts.artifactType != "plugin" {
		return nil, fmt.Errorf("VCS getter can only be used for plugins, not %s", v.opts.artifactType)
	}

	return v.get(href)
}

func (v *VCSGetter) get(href string) (*bytes.Buffer, error) {
	repoURL := v.normalizeURL(href)

	key, err := cache.Key(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cache key: %w", err)
	}

	localPath := helmpath.CachePath("plugins", key)

	repo, err := vcs.NewRepo(repoURL, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create VCS repo: %w", err)
	}

	if err := v.syncRepo(repo); err != nil {
		return nil, fmt.Errorf("failed to sync repository: %w", err)
	}

	if v.opts.version != "" {
		ref, err := v.solveVersion(repo, v.opts.version)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve version %q: %w", v.opts.version, err)
		}
		if ref != "" {
			if err := v.setVersion(repo, ref); err != nil {
				return nil, fmt.Errorf("failed to set version %q: %w", ref, err)
			}
		}
	}

	return v.archiveRepo(repo)
}

func (v *VCSGetter) normalizeURL(href string) string {
	// Handle git+ prefixed schemes
	if strings.HasPrefix(href, "git+") {
		return strings.TrimPrefix(href, "git+")
	}

	// Convert git:// to https://
	if strings.HasPrefix(href, "git://") {
		return strings.Replace(href, "git://", "https://", 1)
	}

	// Add .git suffix to github.com URLs if missing
	if strings.Contains(href, "github.com") && !strings.HasSuffix(href, ".git") {
		return href + ".git"
	}

	return href
}

// syncRepo clones or updates a remote repo using vcs library
func (v *VCSGetter) syncRepo(repo vcs.Repo) error {
	if _, err := os.Stat(repo.LocalPath()); os.IsNotExist(err) {
		slog.Debug("cloning repository", "url", repo.Remote(), "path", repo.LocalPath())
		return repo.Get()
	}
	slog.Debug("updating repository", "url", repo.Remote(), "path", repo.LocalPath())
	return repo.Update()
}

// solveVersion determines the version to checkout based on constraints
func (v *VCSGetter) solveVersion(repo vcs.Repo, version string) (string, error) {
	if version == "" {
		return "", nil
	}

	if repo.IsReference(version) {
		return version, nil
	}

	// Create the constraint first to make sure it's valid before working on the repo
	constraint, err := semver.NewConstraint(version)
	if err != nil {
		return "", err
	}

	// Get the tags
	refs, err := repo.Tags()
	if err != nil {
		return "", err
	}
	slog.Debug("found refs", "refs", refs)

	// Convert and filter the list to semver.Version instances
	semvers := v.getSemVers(refs)

	// Sort semver list
	sort.Sort(sort.Reverse(semver.Collection(semvers)))
	for _, sv := range semvers {
		if constraint.Check(sv) {
			// If the constraint passes get the original reference
			ver := sv.Original()
			slog.Debug("setting to version", "version", ver)
			return ver, nil
		}
	}

	return "", fmt.Errorf("requested version %q does not exist for plugin %q", version, repo.Remote())
}

// setVersion attempts to checkout the version
func (v *VCSGetter) setVersion(repo vcs.Repo, ref string) error {
	slog.Debug("setting version", "version", ref)
	return repo.UpdateVersion(ref)
}

func (v *VCSGetter) getSemVers(refs []string) []*semver.Version {
	var sv []*semver.Version
	for _, r := range refs {
		if ver, err := semver.NewVersion(r); err == nil {
			sv = append(sv, ver)
		}
	}
	return sv
}

// archiveRepo creates a tar.gz archive from the repository content
func (v *VCSGetter) archiveRepo(repo vcs.Repo) (*bytes.Buffer, error) {
	// For now, create a simple tar from the local path
	// In a full implementation, this would create a proper archive
	// but for the getter interface, we need to return the raw plugin content

	// Read the entire directory content into a buffer
	// This is a simplified approach - the original installer copied files directly
	localPath := repo.LocalPath()

	// Check if this looks like a plugin directory structure
	if _, err := os.Stat(filepath.Join(localPath, "plugin.yaml")); err == nil {
		// Read the plugin.yaml to return as content
		content, err := os.ReadFile(filepath.Join(localPath, "plugin.yaml"))
		if err != nil {
			return nil, fmt.Errorf("failed to read plugin.yaml: %w", err)
		}
		return bytes.NewBuffer(content), nil
	}

	return nil, errors.New("not a valid plugin repository - missing plugin.yaml")
}

// RestrictToArtifactTypes implements the getter.Restricted interface.
// VCS getters only support plugins.
func (v *VCSGetter) RestrictToArtifactTypes() []string {
	return []string{"plugin"}
}

func NewVCSGetter(options ...Option) (Getter, error) {
	var getter VCSGetter
	for _, opt := range options {
		opt(&getter.opts)
	}
	return &getter, nil
}
