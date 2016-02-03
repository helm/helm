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

	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// GithubPackageRegistry implements the Registry interface that talks to github and
// expects packages in helm format without versioning and no collection in the path.
// Format of the directory for a type is like so:
// package/
//   Chart.yaml
//   manifests/
//     foo.yaml
//     bar.yaml
//     ...
type GithubPackageRegistry struct {
	githubRegistry
}

// NewGithubPackageRegistry creates a GithubPackageRegistry.
func NewGithubPackageRegistry(name, shortURL string, service GithubRepositoryService, httpClient *http.Client, client *github.Client) (*GithubPackageRegistry, error) {
	format := fmt.Sprintf("%s;%s", common.UnversionedRegistry, common.OneLevelRegistry)
	if service == nil {
		if client == nil {
			service = github.NewClient(nil).Repositories
		} else {
			service = client.Repositories
		}
	}

	gr, err := newGithubRegistry(name, shortURL, common.RegistryFormat(format), httpClient, service)
	if err != nil {
		return nil, err
	}

	return &GithubPackageRegistry{githubRegistry: *gr}, nil
}

// ListTypes lists types in this registry whose string values conform to the
// supplied regular expression, or all types, if the regular expression is nil.
func (g GithubPackageRegistry) ListTypes(regex *regexp.Regexp) ([]Type, error) {
	// Just list all the types at the top level.
	types, err := g.getDirs("")
	if err != nil {
		log.Printf("Failed to list templates: %v", err)
		return nil, err
	}

	var retTypes []Type
	for _, t := range types {
		// Check to see if there's a Chart.yaml file in the directory
		_, dc, _, err := g.service.GetContents(g.owner, g.repository, t, nil)
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

// GetDownloadURLs fetches the download URLs for a given Type.
func (g GithubPackageRegistry) GetDownloadURLs(t Type) ([]*url.URL, error) {
	path, err := g.MakeRepositoryPath(t)
	if err != nil {
		return nil, err
	}

	_, dc, _, err := g.service.GetContents(g.owner, g.repository, path, nil)
	if err != nil {
		log.Printf("Failed to list package files at path: %s: %v", path, err)
		return nil, err
	}

	downloadURLs := []*url.URL{}
	for _, f := range dc {
		if *f.Type == "file" {
			if strings.HasSuffix(*f.Name, ".yaml") {
				u, err := url.Parse(*f.DownloadURL)
				if err != nil {
					return nil, fmt.Errorf("cannot parse URL from %s: %s", *f.DownloadURL, err)
				}

				downloadURLs = append(downloadURLs, u)
			}
		}
	}
	return downloadURLs, nil
}

func (g GithubPackageRegistry) getDirs(dir string) ([]string, error) {
	_, dc, _, err := g.service.GetContents(g.owner, g.repository, dir, nil)
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
func (g GithubPackageRegistry) MakeRepositoryPath(t Type) (string, error) {
	// Construct the return path
	return t.Name + "/manifests", nil
}

func (g GithubPackageRegistry) Do(req *http.Request) (resp *http.Response, err error) {
	return g.httpClient.Do(req)
}
