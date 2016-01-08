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

	"log"
	"strings"
)

// GithubPackageRegistry implements the Registry interface that talks to github and
// expects packages in helm format without versioning and no qualifier in the path.
// Format of the directory for a type is like so:
// package/
//   Chart.yaml
//   manifests/
//     foo.yaml
//     bar.yaml
//     ...
type GithubPackageRegistry struct {
	owner      string
	repository string
	client     *github.Client
}

// NewGithubRegistry creates a Registry that can be used to talk to github.
func NewGithubPackageRegistry(owner, repository string, client *github.Client) *GithubPackageRegistry {
	return &GithubPackageRegistry{
		owner:      owner,
		repository: repository,
		client:     client,
	}
}

// GetRegistryName returns the name of this registry
func (g *GithubPackageRegistry) GetRegistryName() string {
	// TODO(jackgr): implement this method
	return ""
}

// GetRegistryType returns the type of this registry.
func (g *GithubPackageRegistry) GetRegistryType() common.RegistryType {
	// TODO(jackgr): implement this method
	return common.GithubRegistryType
}

// GetRegistryURL returns the URL for this registry.
func (g *GithubPackageRegistry) GetRegistryURL() string {
	// TODO(jackgr): implement this method
	return ""
}

// ListCharts lists the versioned chart names in this registry.
func (g *GithubPackageRegistry) ListCharts() ([]string, error) {
	// TODO(jackgr): implement this method
	return []string{}, nil
}

// GetChart fetches the contents of a given chart.
func (g *GithubPackageRegistry) GetChart(chartName string) (*Chart, error) {
	// TODO(jackgr): implement this method
	return nil, nil
}

// Deprecated: Use ListCharts, instead.
// List the types from the Registry.
func (g *GithubPackageRegistry) List() ([]Type, error) {
	// Just list all the types at the top level.
	types, err := g.getDirs("")
	if err != nil {
		log.Printf("Failed to list templates: %v", err)
		return nil, err
	}

	var retTypes []Type
	for _, t := range types {
		// Check to see if there's a Chart.yaml file in the directory
		_, dc, _, err := g.client.Repositories.GetContents(g.owner, g.repository, t, nil)
		if err != nil {
			log.Printf("Failed to list package files at path: %s: %v", t, err)
			return nil, err
		}
		for _, f := range dc {
			if *f.Type == "file" && *f.Name == "Chart.yaml" {
				retTypes = append(retTypes, Type{Name: t})
			}
		}
	}

	return retTypes, nil
}

// Deprecated: Use GetChart, instead.
// GetURLs fetches the download URLs for a given Type.
func (g *GithubPackageRegistry) GetURLs(t Type) ([]string, error) {
	path, err := g.MakeRepositoryPath(t)
	if err != nil {
		return []string{}, err
	}
	_, dc, _, err := g.client.Repositories.GetContents(g.owner, g.repository, path, nil)
	if err != nil {
		log.Printf("Failed to list package files at path: %s: %v", path, err)
		return []string{}, err
	}
	downloadURLs := []string{}
	for _, f := range dc {
		if *f.Type == "file" {
			if strings.HasSuffix(*f.Name, ".yaml") {
				downloadURLs = append(downloadURLs, *f.DownloadURL)
			}
		}
	}
	return downloadURLs, nil
}

func (g *GithubPackageRegistry) getDirs(dir string) ([]string, error) {
	_, dc, _, err := g.client.Repositories.GetContents(g.owner, g.repository, dir, nil)
	if err != nil {
		log.Printf("Failed to get contents at path: %s: %v", dir, err)
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

// MakeRepositoryPath constructs a github path to a given type based on a repository, and type name.
// The returned repository path will be of the form:
// Type.Name/manifests
func (g *GithubPackageRegistry) MakeRepositoryPath(t Type) (string, error) {
	// Construct the return path
	return t.Name + "/manifests", nil
}
