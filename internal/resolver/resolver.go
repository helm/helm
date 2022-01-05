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

package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/gates"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/provenance"
	"helm.sh/helm/v3/pkg/repo"

	orasregistry "oras.land/oras-go/pkg/registry"
	orasremote "oras.land/oras-go/pkg/registry/remote"
	orasauth "oras.land/oras-go/pkg/registry/remote/auth"
)

const FeatureGateOCI = gates.Gate("HELM_EXPERIMENTAL_OCI")

// Resolver resolves dependencies from semantic version ranges to a particular version.
type Resolver struct {
	chartpath string
	cachepath string
}

// New creates a new resolver for a given chart and a given helm home.
func New(chartpath, cachepath string) *Resolver {
	return &Resolver{
		chartpath: chartpath,
		cachepath: cachepath,
	}
}

// Resolve resolves dependencies and returns a lock file with the resolution.
// To-do: clarify that we use "Repository" in struct as URL for registrytoo, even though strictly speaking it's not a helm "repository"
func (r *Resolver) Resolve(reqs []*chart.Dependency, repoNames map[string]string) (*chart.Lock, error) {

	// Now we clone the dependencies, locking as we go.
	locked := make([]*chart.Dependency, len(reqs))
	missing := []string{}
	for i, d := range reqs {
		constraint, err := semver.NewConstraint(d.Version)
		if err != nil {
			return nil, errors.Wrapf(err, "dependency %q has an invalid version/constraint format", d.Name)
		}

		if d.Repository == "" {
			// Local chart subfolder
			if _, err := GetLocalPath(filepath.Join("charts", d.Name), r.chartpath); err != nil {
				return nil, err
			}

			locked[i] = &chart.Dependency{
				Name:       d.Name,
				Repository: "",
				Version:    d.Version,
			}
			continue
		}
		if strings.HasPrefix(d.Repository, "file://") {

			chartpath, err := GetLocalPath(d.Repository, r.chartpath)
			if err != nil {
				return nil, err
			}

			ch, err := loader.LoadDir(chartpath)
			if err != nil {
				return nil, err
			}

			v, err := semver.NewVersion(ch.Metadata.Version)
			if err != nil {
				// Not a legit entry.
				continue
			}

			if !constraint.Check(v) {
				missing = append(missing, d.Name)
				continue
			}

			locked[i] = &chart.Dependency{
				Name:       d.Name,
				Repository: d.Repository,
				Version:    ch.Metadata.Version,
			}
			continue
		}

		repoName := repoNames[d.Name]
		// if the repository was not defined, but the dependency defines a repository url, bypass the cache
		if repoName == "" && d.Repository != "" {
			locked[i] = &chart.Dependency{
				Name:       d.Name,
				Repository: d.Repository,
				Version:    d.Version,
			}
			continue
		}

		var vs repo.ChartVersions
		var version string
		var ok bool
		found := true
		if !registry.IsOCI(d.Repository) {
			repoIndex, err := repo.LoadIndexFile(filepath.Join(r.cachepath, helmpath.CacheIndexFile(repoName)))
			if err != nil {
				return nil, errors.Wrapf(err, "no cached repository for %s found. (try 'helm repo update')", repoName)
			}

			vs, ok = repoIndex.Entries[d.Name]
			if !ok {
				return nil, errors.Errorf("%s chart not found in repo %s", d.Name, d.Repository)
			}
			found = false
		} else {
			version = d.Version
			if !FeatureGateOCI.IsEnabled() {
				return nil, errors.Wrapf(FeatureGateOCI.Error(),
					"repository %s is an OCI registry", d.Repository)
			}

			// Call ORAS tag API
			// See https://github.com/oras-project/oras-go/pull/89
			// 	- using string: concat d.Repository + d.Name
			// 	- does latest version exist, find out how Masterminds/semver checks this given contstraint string, and get the version
			// To-do: use registry.ctx()
			// 	How to get the opts though without context from a *Client?
			ctx := context.Background()
			// Do we need to extract something from this string in order to work?
			ociRepository := d.Repository
			parsedRepository, err := orasregistry.ParseReference(ociRepository)
			if err != nil {
				return nil, errors.Wrapf(err, "no cached repository for %s found. (try 'helm repo update')", repoName)
			}

			// To:do: Can we get client values from registry.NewClient()?
			// 	 If so how to pass client ops without an existing *Client?
			// Example code for this:
			// client, err := registry.NewClient()
			// if err != nil {
			// 	return nil, err
			// }
			client := &orasauth.Client{
				Header: http.Header{
					"User-Agent": {"oras-go"},
				},
				Cache: orasauth.DefaultCache,
			}

			repository := orasremote.Repository{
				Reference: parsedRepository,
				Client:    client,
			}

			// Get tags from ORAS tag API
			// To-do: The block farther below comment says
			//   "The version are already sorted and hence the first one to satisfy the constraint is used"
			// 	 So how do we ensure these are sorted?
			tags, err := orasregistry.Tags(ctx, &repository)
			if err != nil {
				return nil, err
			}

			// Mock chart version objects
			vs = make(repo.ChartVersions, len(tags))
			for i, tag := range tags {
				// Change underscore (_) back to plus (+) for Helm
				// See https://github.com/helm/helm/issues/10166
				tag = strings.ReplaceAll(tag, "_", "+")

				// Then add those on what comes from equivalent of tags
				// To-do: do we need anything here other than Version and Name?
				vs[i].Version = tag
				vs[i].Name = d.Name
			}

			// Ultimately either here before continue, or below, we want to accomplish:
			// 	 1. if there's a reference that matches the version (did it work or not? could it locate image? 401 or 403 etc)
			// 	 2. if so, proceed
			// But below, if `len(ver.URLs) == 0` it's not considered valid, so we continue
			// what is the equivalent of this for OCI? There is no tarball URL(s), right?
		}

		locked[i] = &chart.Dependency{
			Name:       d.Name,
			Repository: d.Repository,
			Version:    version,
		}
		// The version are already sorted and hence the first one to satisfy the constraint is used
		for _, ver := range vs {
			v, err := semver.NewVersion(ver.Version)
			if err != nil || len(ver.URLs) == 0 {
				// Not a legit entry.
				continue
			}
			if constraint.Check(v) {
				found = true
				locked[i].Version = v.Original()
				break
			}
		}

		if !found {
			missing = append(missing, d.Name)
		}
	}
	if len(missing) > 0 {
		return nil, errors.Errorf("can't get a valid version for repositories %s. Try changing the version constraint in Chart.yaml", strings.Join(missing, ", "))
	}

	digest, err := HashReq(reqs, locked)
	if err != nil {
		return nil, err
	}

	return &chart.Lock{
		Generated:    time.Now(),
		Digest:       digest,
		Dependencies: locked,
	}, nil
}

// HashReq generates a hash of the dependencies.
//
// This should be used only to compare against another hash generated by this
// function.
func HashReq(req, lock []*chart.Dependency) (string, error) {
	data, err := json.Marshal([2][]*chart.Dependency{req, lock})
	if err != nil {
		return "", err
	}
	s, err := provenance.Digest(bytes.NewBuffer(data))
	return "sha256:" + s, err
}

// HashV2Req generates a hash of requirements generated in Helm v2.
//
// This should be used only to compare against another hash generated by the
// Helm v2 hash function. It is to handle issue:
// https://github.com/helm/helm/issues/7233
func HashV2Req(req []*chart.Dependency) (string, error) {
	dep := make(map[string][]*chart.Dependency)
	dep["dependencies"] = req
	data, err := json.Marshal(dep)
	if err != nil {
		return "", err
	}
	s, err := provenance.Digest(bytes.NewBuffer(data))
	return "sha256:" + s, err
}

// GetLocalPath generates absolute local path when use
// "file://" in repository of dependencies
func GetLocalPath(repo, chartpath string) (string, error) {
	var depPath string
	var err error
	p := strings.TrimPrefix(repo, "file://")

	// root path is absolute
	if strings.HasPrefix(p, "/") {
		if depPath, err = filepath.Abs(p); err != nil {
			return "", err
		}
	} else {
		depPath = filepath.Join(chartpath, p)
	}

	if _, err = os.Stat(depPath); os.IsNotExist(err) {
		return "", errors.Errorf("directory %s not found", depPath)
	} else if err != nil {
		return "", err
	}

	return depPath, nil
}
