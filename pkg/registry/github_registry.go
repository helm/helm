/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package registry

import (
	"github.com/google/go-github/github"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/util"

	"fmt"
	"net/http"
	"strings"
)

// githubRegistry is the base class for the Registry interface and talks to github. Actual implementations are
// in GithubPackageRegistry and GithubTemplateRegistry.
// The registry short URL and format determine how types are laid out in the
// registry.
type githubRegistry struct {
	name           string
	shortURL       string
	owner          string
	repository     string
	path           string
	format         common.RegistryFormat
	credentialName string
	service        GithubRepositoryService
	httpClient     *http.Client
}

// GithubRepositoryService defines the interface that's defined in github.com/go-github/repos_contents.go GetContents method.
type GithubRepositoryService interface {
	GetContents(
		owner, repo, path string,
		opt *github.RepositoryContentGetOptions,
	) (
		fileContent *github.RepositoryContent,
		directoryContent []*github.RepositoryContent,
		resp *github.Response,
		err error,
	)
}

// newGithubRegistry creates a githubRegistry.
func newGithubRegistry(name, shortURL string, format common.RegistryFormat, httpClient *http.Client, service GithubRepositoryService) (*githubRegistry, error) {
	trimmed := util.TrimURLScheme(shortURL)
	owner, repository, path, err := parseGithubShortURL(trimmed)
	if err != nil {
		return nil, fmt.Errorf("cannot create Github template registry %s: %s", name, err)
	}

	return &githubRegistry{
		name:       name,
		shortURL:   trimmed,
		owner:      owner,
		repository: repository,
		path:       path,
		format:     format,
		service:    service,
		httpClient: httpClient,
	}, nil
}

func parseGithubShortURL(shortURL string) (string, string, string, error) {
	if !strings.HasPrefix(shortURL, "github.com/") {
		return "", "", "", fmt.Errorf("invalid Github short URL: %s", shortURL)
	}

	tPath := strings.TrimPrefix(shortURL, "github.com/")
	parts := strings.Split(tPath, "/")

	// Handle the case where there's no path after owner and repository.
	if len(parts) == 2 {
		return parts[0], parts[1], "", nil
	}

	// Handle the case where there's a path after owner and repository.
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2], nil
	}

	return "", "", "", fmt.Errorf("invalid Github short URL: %s", shortURL)
}

// GetRegistryName returns the name of this registry
func (g githubRegistry) GetRegistryName() string {
	return g.name
}

// GetRegistryType returns the type of this registry.
func (g githubRegistry) GetRegistryType() common.RegistryType {
	return common.GithubRegistryType
}

// GetRegistryShortURL returns the short URL for this registry.
func (g githubRegistry) GetRegistryShortURL() string {
	return g.shortURL
}

// GetRegistryFormat returns the format of this registry.
func (g githubRegistry) GetRegistryFormat() common.RegistryFormat {
	return g.format
}

// GetRegistryOwner returns the owner name for this registry
func (g githubRegistry) GetRegistryOwner() string {
	return g.owner
}

// GetRegistryRepository returns the repository name for this registry.
func (g githubRegistry) GetRegistryRepository() string {
	return g.repository
}

// GetRegistryName returns the name of this registry
func (g githubRegistry) GetRegistryPath() string {
	return g.path
}
