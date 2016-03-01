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
	"github.com/kubernetes/deployment-manager/pkg/common"

	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// GithubTemplateRegistry implements the Registry interface and implements a
// Deployment Manager templates registry.
// A registry root must be a directory that contains all the available templates,
// one directory per template. Each template directory then contains version
// directories, each of which in turn contains all the files necessary for that
// version of the template.
//
// For example, a template registry containing two versions of redis
// (implemented in jinja), and one version of replicatedservice (implemented
// in python) would have a directory structure that looks something like this:
// qualifier [optional] prefix to a virtual root within the repository.
// /redis
//   /v1
//     redis.jinja
//     redis.jinja.schema
//   /v2
//     redis.jinja
//     redis.jinja.schema
// /replicatedservice
//   /v1
//     replicatedservice.python
//     replicatedservice.python.schema
type GithubTemplateRegistry struct {
	githubRegistry
}

// NewGithubTemplateRegistry creates a GithubTemplateRegistry.
func NewGithubTemplateRegistry(name, shortURL string, service GithubRepositoryService, httpClient *http.Client, client *github.Client) (*GithubTemplateRegistry, error) {
	format := fmt.Sprintf("%s;%s", common.VersionedRegistry, common.CollectionRegistry)
	if service == nil {
		service = client.Repositories
	}

	gr, err := newGithubRegistry(name, shortURL, common.RegistryFormat(format), httpClient, service)
	if err != nil {
		return nil, err
	}

	return &GithubTemplateRegistry{githubRegistry: *gr}, nil
}

// ListTypes lists types in this registry whose string values conform to the
// supplied regular expression, or all types, if the regular expression is nil.
func (g GithubTemplateRegistry) ListTypes(regex *regexp.Regexp) ([]Type, error) {
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

	if regex != nil {
		var matchTypes []Type
		for _, retType := range retTypes {
			if regex.MatchString(retType.String()) {
				matchTypes = append(matchTypes, retType)
			}
		}

		return matchTypes, nil
	}

	return retTypes, nil
}

// GetDownloadURLs fetches the download URLs for a given Type and checks for existence of a schema file.
func (g GithubTemplateRegistry) GetDownloadURLs(t Type) ([]*url.URL, error) {
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

func (g GithubTemplateRegistry) getDirs(dir string) ([]string, error) {
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

func (g GithubTemplateRegistry) mapCollection(collection string) (string, error) {
	if strings.ContainsAny(collection, "/") {
		return "", fmt.Errorf("collection must not contain slashes, got %s", collection)
	}
	// TODO(vaikas): Implement lookup from the root metadata file to map collection to a path
	return collection, nil
}

// MakeRepositoryPath constructs a github path to a given type based on a repository, and type name and version.
// The returned repository path will be of the form:
// [GithubTemplateRegistry.path/][Type.Collection]/Type.Name/Type.Version
// Type.Collection will be mapped using mapCollection in the future, for now it's a straight
// 1:1 mapping (if given)
func (g GithubTemplateRegistry) MakeRepositoryPath(t Type) (string, error) {
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

// Do performs an HTTP operation on the receiver's httpClient.
func (g GithubTemplateRegistry) Do(req *http.Request) (resp *http.Response, err error) {
	return g.httpClient.Do(req)
}
