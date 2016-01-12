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
	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/util"

	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
)

// githubRegistry implements the Registry interface and talks to github.
// The registry short URL and format determine how types are laid out in the
// registry.
type githubRegistry struct {
	name       string
	shortURL   string
	owner      string
	repository string
	path       string
	format     common.RegistryFormat
	service    RepositoryService
}

type RepositoryService interface {
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
func newGithubRegistry(name, shortURL string, format common.RegistryFormat, service RepositoryService) (githubRegistry, error) {
	trimmed := util.TrimURLScheme(shortURL)
	owner, repository, path, err := parseGithubShortURL(trimmed)
	if err != nil {
		return githubRegistry{}, fmt.Errorf("cannot create Github template registry %s: %s", name, err)
	}

	if service == nil {
		client := github.NewClient(nil)
		service = client.Repositories
	}

	return githubRegistry{
		name:       name,
		shortURL:   trimmed,
		owner:      owner,
		repository: repository,
		path:       path,
		format:     format,
		service:    service,
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

// ListTypes lists types in this registry whose string values conform to the
// supplied regular expression, or all types, if the regular expression is nil.
func (g githubRegistry) ListTypes(regex *regexp.Regexp) ([]Type, error) {
	// First list all the collections at the top level.
	collections, err := g.getDirs("")
	if err != nil {
		log.Printf("cannot list qualifiers: %v", err)
		return nil, err
	}

	var retTypes []Type
	for _, c := range collections {
		// Then we need to fetch the versions (directories for this type)
		types, err := g.getDirs(c)
		if err != nil {
			log.Printf("cannot fetch types for collection: %s", c)
			return nil, err
		}

		for _, t := range types {
			path := c + "/" + t
			// Then we need to fetch the versions (directories for this type)
			versions, err := g.getDirs(path)
			if err != nil {
				log.Printf("cannot fetch versions at path %s", path)
				return nil, err
			}

			for _, v := range versions {
				tt, err := NewType(c, t, v)
				if err != nil {
					return nil, fmt.Errorf("malformed type at path %s", path)
				}

				retTypes = append(retTypes, tt)
			}
		}
	}

	// TODO(jackgr): Use the supplied regex to filter the results.
	return retTypes, nil
}

// GetDownloadURLs fetches the download URLs for a given Type and checks for existence of a schema file.
func (g githubRegistry) GetDownloadURLs(t Type) ([]*url.URL, error) {
	path, err := g.MakeRepositoryPath(t)
	if err != nil {
		return nil, err
	}
	_, dc, _, err := g.service.GetContents(g.owner, g.repository, path, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot list versions at path %s: %v", path, err)
	}

	var downloadURL, typeName, schemaName string
	for _, f := range dc {
		if *f.Type == "file" {
			if *f.Name == t.Name+".jinja" || *f.Name == t.Name+".py" {
				typeName = *f.Name
				downloadURL = *f.DownloadURL
			}
			if *f.Name == t.Name+".jinja.schema" || *f.Name == t.Name+".py.schema" {
				schemaName = *f.Name
			}
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("cannot find type %s", t.String())
	}

	if schemaName != typeName+".schema" {
		return nil, fmt.Errorf("cannot find schema for %s, expected %s", t.String(), typeName+".schema")
	}

	result, err := url.Parse(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URL from %s: %s", downloadURL, err)
	}

	return []*url.URL{result}, nil
}

func (g githubRegistry) getDirs(dir string) ([]string, error) {
	var path = g.path
	if dir != "" {
		path = g.path + "/" + dir
	}

	_, dc, _, err := g.service.GetContents(g.owner, g.repository, path, nil)
	if err != nil {
		log.Printf("Failed to get contents at path: %s: %v", path, err)
		return nil, err
	}

	var dirs []string
	for _, entry := range dc {
		if *entry.Type == "dir" {
			dirs = append(dirs, *entry.Name)
		}
	}

	return dirs, nil
}

func (g githubRegistry) mapCollection(collection string) (string, error) {
	if strings.ContainsAny(collection, "/") {
		return "", fmt.Errorf("collection must not contain slashes, got %s", collection)
	}
	// TODO(vaikas): Implement lookup from the root metadata file to map collection to a path
	return collection, nil
}

// MakeRepositoryPath constructs a github path to a given type based on a repository, and type name and version.
// The returned repository path will be of the form:
// [githubRegistry.path/][Type.Collection]/Type.Name/Type.Version
// Type.Collection will be mapped using mapCollection in the future, for now it's a straight
// 1:1 mapping (if given)
func (g githubRegistry) MakeRepositoryPath(t Type) (string, error) {
	// First map the collection
	collection, err := g.mapCollection(t.Collection)
	if err != nil {
		return "", err
	}
	// Construct the return path
	p := ""
	if len(g.path) > 0 {
		p += g.path + "/"
	}
	if len(collection) > 0 {
		p += collection + "/"
	}
	return p + t.Name + "/" + t.GetVersion(), nil
}
