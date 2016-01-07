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

	"fmt"
	"log"
	"strings"
)

// GithubRegistry implements the Registry interface that talks to github and
// implements Deployment Manager templates registry.
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

type GithubRegistry struct {
	owner      string
	repository string
	path       string
	client     *github.Client
}

// NewGithubRegistry creates a Registry that can be used to talk to github.
func NewGithubRegistry(owner, repository, path string, client *github.Client) *GithubRegistry {
	return &GithubRegistry{
		owner:      owner,
		repository: repository,
		path:       path,
		client:     client,
	}
}

// List the types from the Registry.
func (g *GithubRegistry) List() ([]Type, error) {
	// First list all the collections at the top level.
	collections, err := g.getDirs("")
	if err != nil {
		log.Printf("Failed to list qualifiers: %v", err)
		return nil, err
	}

	var retTypes []Type
	for _, c := range collections {
		// Then we need to fetch the versions (directories for this type)
		types, err := g.getDirs(c)
		if err != nil {
			log.Printf("Failed to fetch types for collection: %s", c)
			return nil, err
		}

		for _, t := range types {
			// Then we need to fetch the versions (directories for this type)
			versions, err := g.getDirs(c + "/" + t)
			if err != nil {
				log.Printf("Failed to fetch versions for template: %s", t)
				return nil, err
			}
			for _, v := range versions {
				retTypes = append(retTypes, Type{Name: t, Version: v, Collection: c})
			}
		}
	}

	return retTypes, nil
}

// GetURL fetches the download URL for a given Type and checks for existence of a schema file.
func (g *GithubRegistry) GetURLs(t Type) ([]string, error) {
	path, err := g.MakeRepositoryPath(t)
	if err != nil {
		return []string{}, err
	}
	_, dc, _, err := g.client.Repositories.GetContents(g.owner, g.repository, path, nil)
	if err != nil {
		log.Printf("Failed to list versions at path: %s: %v", path, err)
		return []string{}, err
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
		return []string{}, fmt.Errorf("Can not find template %s:%s", t.Name, t.Version)
	}
	if schemaName == typeName+".schema" {
		return []string{downloadURL}, nil
	}
	return []string{}, fmt.Errorf("Can not find schema for %s:%s, expected to find %s", t.Name, t.Version, typeName+".schema")
}

func (g *GithubRegistry) getDirs(dir string) ([]string, error) {
	var path = g.path
	if dir != "" {
		path = g.path + "/" + dir
	}

	_, dc, _, err := g.client.Repositories.GetContents(g.owner, g.repository, path, nil)
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

func (g *GithubRegistry) mapCollection(collection string) (string, error) {
	if strings.ContainsAny(collection, "/") {
		return "", fmt.Errorf("collection must not contain slashes, got %s", collection)
	}
	// TODO(vaikas): Implement lookup from the root metadata file to map collection to a path
	return collection, nil
}

// MakeRepositoryPath constructs a github path to a given type based on a repository, and type name and version.
// The returned repository path will be of the form:
// [GithubRegistry.path/][Type.Collection]/Type.Name/Type.Version
// Type.Collection will be mapped using mapCollection in the future, for now it's a straight
// 1:1 mapping (if given)
func (g *GithubRegistry) MakeRepositoryPath(t Type) (string, error) {
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
	return p + t.Name + "/" + t.Version, nil
}
