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
)

type GithubRegistry struct {
	owner      string
	repository string
	root       string
	client     *github.Client
}

func NewGithubRegistry(owner string, repository string, root string) *GithubRegistry {
	return &GithubRegistry{
		owner:      owner,
		repository: repository,
		root:       root,
		client:     github.NewClient(nil),
	}
}

func (g *GithubRegistry) List() ([]Type, error) {
	log.Printf("Calling ListRefs")
	// First list all the types at the top level.
	types, err := g.getDirs(TypesDir)
	if err != nil {
		log.Printf("Failed to list types : %v", err)
		return nil, err
	}
	var retTypes []Type
	for _, t := range types {
		log.Printf("Got TYPE: %s, fetching : %s", t, TypesDir+"/"+t)
		// Then we need to fetch the versions (directories for this type)
		versions, err := g.getDirs(TypesDir + "/" + t)
		if err != nil {
			log.Printf("Failed to fetch versions for type: %s", t)
			return nil, err
		}
		for _, v := range versions {
			log.Printf("Got VERSION: %s", v)
			retTypes = append(retTypes, Type{Name: t, Version: v})
		}
	}

	return retTypes, nil
}

// Get the URL for a given type
func (g *GithubRegistry) GetURL(t Type) (string, error) {
	_, dc, _, err := g.client.Repositories.GetContents(g.owner, g.repository, TypesDir+"/"+t.Name+"/"+t.Version, nil)
	if err != nil {
		log.Printf("Failed to list types : %v", err)
		return "", err
	}
	for _, f := range dc {
		if *f.Type == "file" {
			if *f.Name == t.Version+".jinja" || *f.Name == t.Version+".py" {
				return *f.DownloadURL, nil
			}
		}
	}
	return "", fmt.Errorf("Can not find type %s:%s", t.Name, t.Version)
}

func (g *GithubRegistry) getDirs(dir string) ([]string, error) {
	_, dc, resp, err := g.client.Repositories.GetContents(g.owner, g.repository, dir, nil)
	if err != nil {
		log.Printf("Failed to call ListRefs : %v", err)
		return nil, err
	}
	log.Printf("Got: %v %v", dc, resp, err)
	var dirs []string
	for _, entry := range dc {
		if *entry.Type == "dir" {
			dirs = append(dirs, *entry.Name)
		}
	}
	return dirs, nil
}
