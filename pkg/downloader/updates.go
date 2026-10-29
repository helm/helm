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

package downloader

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"helm.sh/helm/v4/internal/resolver"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
)

// DependencyUpdate compares the lock file with versions available at the source.
type DependencyUpdate struct {
	Chart      string `json:"chart" yaml:"chart"`
	Name       string `json:"name" yaml:"name"`
	Repository string `json:"repository" yaml:"repository"`
	Constraint string `json:"constraint" yaml:"constraint"`
	Locked     string `json:"locked" yaml:"locked"`
	Compatible string `json:"compatible" yaml:"compatible"`
	Latest     string `json:"latest" yaml:"latest"`
}

// CheckUpdates refreshes repository metadata unless SkipUpdate is set. It does
// not download chart archives or change Chart.yaml, Chart.lock, or charts/.
func (m *Manager) CheckUpdates() ([]DependencyUpdate, error) {
	c, err := m.loadChartDir()
	if err != nil {
		return nil, err
	}
	results := make([]DependencyUpdate, 0, len(c.Metadata.Dependencies))
	if len(c.Metadata.Dependencies) == 0 {
		return results, nil
	}
	repoNames := make([]string, len(c.Metadata.Dependencies))
	hasRemote := false
	originalRepositories := make([]string, len(c.Metadata.Dependencies))
	for i, dependency := range c.Metadata.Dependencies {
		originalRepositories[i] = dependency.Repository
		if dependency.Repository != "" && !strings.HasPrefix(dependency.Repository, "file://") && !registry.IsOCI(dependency.Repository) {
			hasRemote = true
			// Different aliases can refer to identically named charts in
			// different repositories. Resolve each declaration independently.
			deps := []*chart.Dependency{dependency}
			names, err := m.resolveRepoNames(deps)
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(dependency.Repository, "@") || strings.HasPrefix(dependency.Repository, "alias:") {
				return nil, fmt.Errorf("no repository definition for %s; add it with 'helm repo add'", dependency.Repository)
			}
			names, err = m.ensureMissingRepos(names, deps)
			if err != nil {
				return nil, err
			}
			repoNames[i] = names[dependency.Name]
		}
	}
	if !m.SkipUpdate && hasRemote {
		if err := m.UpdateRepositories(); err != nil {
			return nil, err
		}
	}
	for i, dependency := range c.Metadata.Dependencies {
		constraint, err := semver.NewConstraint(dependency.Version)
		if err != nil {
			return nil, fmt.Errorf("dependency %q has an invalid version constraint: %w", dependency.Name, err)
		}
		versions, err := m.dependencyVersions(c, dependency, repoNames[i])
		if err != nil {
			return nil, fmt.Errorf("check dependency %q: %w", dependency.Name, err)
		}
		compatible, latest := newestDependencyVersions(versions, constraint)
		name := dependency.Name
		if dependency.Alias != "" {
			name = dependency.Alias
		}
		results = append(results, DependencyUpdate{
			Chart:      m.ChartPath,
			Name:       name,
			Repository: originalRepositories[i],
			Constraint: dependency.Version,
			Locked:     lockedDependencyVersion(c.Lock, dependency, originalRepositories[i], i),
			Compatible: compatible,
			Latest:     latest,
		})
	}
	return results, nil
}

func (m *Manager) dependencyVersions(parent *chart.Chart, dependency *chart.Dependency, repoName string) ([]string, error) {
	switch {
	case dependency.Repository == "":
		for _, child := range parent.Dependencies() {
			if child.Name() == dependency.Name || (dependency.Alias != "" && child.Name() == dependency.Alias) {
				return []string{child.Metadata.Version}, nil
			}
		}
		return nil, fmt.Errorf("local chart %s is missing from charts/", dependency.Name)
	case strings.HasPrefix(dependency.Repository, "file://"):
		path, err := resolver.GetLocalPath(dependency.Repository, m.ChartPath)
		if err != nil {
			return nil, err
		}
		local, err := loader.LoadDir(path)
		if err != nil {
			return nil, err
		}
		return []string{local.Metadata.Version}, nil
	case registry.IsOCI(dependency.Repository):
		if m.RegistryClient == nil {
			return nil, fmt.Errorf("an OCI registry client is required for %s", dependency.Repository)
		}
		ref := strings.TrimSuffix(strings.TrimPrefix(dependency.Repository, registry.OCIScheme+"://"), "/") + "/" + dependency.Name
		return m.RegistryClient.Tags(ref)
	default:
		index, err := repo.LoadIndexFile(filepath.Join(m.RepositoryCache, helmpath.CacheIndexFile(repoName)))
		if err != nil {
			return nil, fmt.Errorf("load repository index (try 'helm repo update'): %w", err)
		}
		entries, ok := index.Entries[dependency.Name]
		if !ok {
			return nil, fmt.Errorf("chart %s not found in repository %s", dependency.Name, dependency.Repository)
		}
		versions := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry != nil && entry.Metadata != nil && len(entry.URLs) > 0 {
				versions = append(versions, entry.Version)
			}
		}
		return versions, nil
	}
}

func newestDependencyVersions(versions []string, constraint *semver.Constraints) (compatible, latest string) {
	var compatibleVersion, latestVersion *semver.Version
	for _, raw := range versions {
		version, err := semver.NewVersion(raw)
		if err != nil {
			continue
		}
		if version.Prerelease() == "" && (latestVersion == nil || version.GreaterThan(latestVersion)) {
			latestVersion, latest = version, raw
		}
		if constraint.Check(version) && (compatibleVersion == nil || version.GreaterThan(compatibleVersion)) {
			compatibleVersion, compatible = version, raw
		}
	}
	return compatible, latest
}

func lockedDependencyVersion(lock *chart.Lock, dependency *chart.Dependency, originalRepository string, position int) string {
	if lock == nil {
		return ""
	}
	matches := func(locked *chart.Dependency) bool {
		return locked != nil && locked.Name == dependency.Name &&
			(locked.Repository == dependency.Repository || locked.Repository == originalRepository)
	}
	// Lock entries normally preserve declaration order, including aliases of
	// the same chart with different version constraints.
	if position < len(lock.Dependencies) && matches(lock.Dependencies[position]) {
		return lock.Dependencies[position].Version
	}
	var found string
	for _, locked := range lock.Dependencies {
		if matches(locked) {
			if found != "" {
				return "" // A stale lock cannot disambiguate multiple aliases.
			}
			found = locked.Version
		}
	}
	return found
}
