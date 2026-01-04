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

package installer // import "helm.sh/helm/v4/internal/plugin/installer"

import (
	"errors"
	"fmt"
	stdfs "io/fs"
	"log/slog"
	"os"
	"os/exec"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/Masterminds/vcs"

	"helm.sh/helm/v4/internal/plugin/cache"
	"helm.sh/helm/v4/internal/third_party/dep/fs"
	"helm.sh/helm/v4/pkg/helmpath"
)

// VCSInstaller installs plugins from remote a repository.
type VCSInstaller struct {
	Repo    vcs.Repo
	Version string
	base
}

func existingVCSRepo(location string) (Installer, error) {
	repo, err := vcs.NewRepo("", location)
	if err != nil {
		return nil, err
	}
	i := &VCSInstaller{
		Repo: repo,
		base: newBase(repo.Remote()),
	}
	return i, nil
}

// NewVCSInstaller creates a new VCSInstaller.
func NewVCSInstaller(source, version string) (*VCSInstaller, error) {
	key, err := cache.Key(source)
	if err != nil {
		return nil, err
	}
	cachedpath := helmpath.CachePath("plugins", key)
	repo, err := vcs.NewRepo(source, cachedpath)
	if err != nil {
		return nil, err
	}
	i := &VCSInstaller{
		Repo:    repo,
		Version: version,
		base:    newBase(source),
	}
	return i, nil
}

// Install clones a remote repository and installs into the plugin directory.
//
// Implements Installer.
func (i *VCSInstaller) Install() error {
	if err := i.sync(i.Repo); err != nil {
		return err
	}

	ref, err := i.solveVersion(i.Repo)
	if err != nil {
		return err
	}
	if ref != "" {
		if err := i.setVersion(i.Repo, ref); err != nil {
			return err
		}
	}

	if !isPlugin(i.Repo.LocalPath()) {
		return ErrMissingMetadata
	}

	slog.Debug("copying files", "source", i.Repo.LocalPath(), "destination", i.Path())
	return fs.CopyDir(i.Repo.LocalPath(), i.Path())
}

// resetPluginYaml discards local modifications to plugin.yaml file.
// This is used to clean the cached repository before updating.
// plugin.yaml is the only file that Helm modifies during installation.
func resetPluginYaml(repo vcs.Repo) error {
	pluginYaml := "plugin.yaml"

	// Check the VCS type to determine the appropriate reset command
	switch repo.Vcs() {
	case vcs.Git:
		// For Git, use 'git checkout -- plugin.yaml' to discard changes to this file
		cmd := exec.Command("git", "checkout", "--", pluginYaml)
		cmd.Dir = repo.LocalPath()
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git checkout failed: %w, output: %s", err, output)
		}
		return nil
	case vcs.Hg:
		// For Mercurial, use 'hg revert --no-backup plugin.yaml'
		cmd := exec.Command("hg", "revert", "--no-backup", pluginYaml)
		cmd.Dir = repo.LocalPath()
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("hg revert failed: %w, output: %s", err, output)
		}
		return nil
	case vcs.Bzr:
		// For Bazaar, use 'bzr revert plugin.yaml'
		cmd := exec.Command("bzr", "revert", pluginYaml)
		cmd.Dir = repo.LocalPath()
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bzr revert failed: %w, output: %s", err, output)
		}
		return nil
	case vcs.Svn:
		// For SVN, use 'svn revert plugin.yaml'
		cmd := exec.Command("svn", "revert", pluginYaml)
		cmd.Dir = repo.LocalPath()
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("svn revert failed: %w, output: %s", err, output)
		}
		return nil
	default:
		return fmt.Errorf("unsupported VCS type: %v", repo.Vcs())
	}
}

// Update updates a remote repository
func (i *VCSInstaller) Update() error {
	slog.Debug("updating", "source", i.Repo.Remote())

	// Reset plugin.yaml if it was modified by Helm during installation.
	// The cached repository is managed by Helm and should not contain user modifications.
	// plugin.yaml is the only file that Helm modifies during installation,
	// so we only need to reset this specific file.
	if i.Repo.IsDirty() {
		slog.Debug("resetting plugin.yaml in cache", "path", i.Repo.LocalPath())
		if err := resetPluginYaml(i.Repo); err != nil {
			return fmt.Errorf("failed to reset plugin.yaml: %w", err)
		}
	}

	if err := i.Repo.Update(); err != nil {
		return err
	}
	if !isPlugin(i.Repo.LocalPath()) {
		return ErrMissingMetadata
	}
	return nil
}

func (i *VCSInstaller) solveVersion(repo vcs.Repo) (string, error) {
	if i.Version == "" {
		return "", nil
	}

	if repo.IsReference(i.Version) {
		return i.Version, nil
	}

	// Create the constraint first to make sure it's valid before
	// working on the repo.
	constraint, err := semver.NewConstraint(i.Version)
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
	semvers := getSemVers(refs)

	// Sort semver list
	sort.Sort(sort.Reverse(semver.Collection(semvers)))
	for _, v := range semvers {
		if constraint.Check(v) {
			// If the constraint passes get the original reference
			ver := v.Original()
			slog.Debug("setting to version", "version", ver)
			return ver, nil
		}
	}

	return "", fmt.Errorf("requested version %q does not exist for plugin %q", i.Version, i.Repo.Remote())
}

// setVersion attempts to checkout the version
func (i *VCSInstaller) setVersion(repo vcs.Repo, ref string) error {
	slog.Debug("setting version", "version", i.Version)
	return repo.UpdateVersion(ref)
}

// sync will clone or update a remote repo.
func (i *VCSInstaller) sync(repo vcs.Repo) error {
	if _, err := os.Stat(repo.LocalPath()); errors.Is(err, stdfs.ErrNotExist) {
		slog.Debug("cloning", "source", repo.Remote(), "destination", repo.LocalPath())
		return repo.Get()
	}
	slog.Debug("updating", "source", repo.Remote(), "destination", repo.LocalPath())
	return repo.Update()
}

// Filter a list of versions to only included semantic versions. The response
// is a mapping of the original version to the semantic version.
func getSemVers(refs []string) []*semver.Version {
	var sv []*semver.Version
	for _, r := range refs {
		if v, err := semver.NewVersion(r); err == nil {
			sv = append(sv, v)
		}
	}
	return sv
}
