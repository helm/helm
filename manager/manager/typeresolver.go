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

package manager

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/kubernetes/deployment-manager/common"
	"github.com/kubernetes/deployment-manager/registry"
	"github.com/kubernetes/deployment-manager/util"

	"github.com/ghodss/yaml"
)

const (
	maxURLImports = 100
	schemaSuffix  = ".schema"
)

// TypeResolver finds Types in a Configuration which aren't yet reduceable to an import file
// or primitive, and attempts to replace them with a template from a URL.
type TypeResolver interface {
	ResolveTypes(config *common.Configuration, imports []*common.ImportFile) ([]*common.ImportFile, error)
}

type typeResolver struct {
	getter  util.HTTPClient
	maxUrls int
	rp      registry.RegistryProvider
}

type fetchUnit struct {
	urls []string
}

// NewTypeResolver returns a new initialized TypeResolver.
func NewTypeResolver() TypeResolver {
	ret := &typeResolver{}
	client := http.DefaultClient
	//TODO (iantw): Make this a flag
	timeout, _ := time.ParseDuration("10s")
	client.Timeout = timeout
	ret.getter = util.NewHTTPClient(3, client, util.NewSleeper())
	ret.maxUrls = maxURLImports
	ret.rp = &registry.DefaultRegistryProvider{}
	return ret
}

func resolverError(c *common.Configuration, err error) error {
	return fmt.Errorf("cannot resolve types in configuration %s due to: \n%s\n",
		c, err)
}

func performHTTPGet(g util.HTTPClient, u string, allowMissing bool) (content string, err error) {
	log.Printf("Fetching %s", u)
	r, code, err := g.Get(u)
	if err != nil {
		return "", err
	}

	if allowMissing && code == http.StatusNotFound {
		return "", nil
	}

	if code != http.StatusOK {
		return "", fmt.Errorf(
			"Received status code %d attempting to fetch Type at %s", code, u)
	}

	return r, nil
}

// ResolveTypes resolves the types in the supplied configuration and returns
// resolved type definitions in t.ImportFiles. Types can be either
// primitive (i.e., built in), resolved (i.e., already t.ImportFiles), or remote
// (i.e., described by a URL that must be fetched to resolve the type).
func (tr *typeResolver) ResolveTypes(config *common.Configuration, imports []*common.ImportFile) ([]*common.ImportFile, error) {
	existing := map[string]bool{}
	for _, v := range imports {
		existing[v.Name] = true
	}

	fetched := map[string][]*common.ImportFile{}
	// TODO(vaikas): Need to account for multiple URLs being fetched for a given type.
	toFetch := make([]*fetchUnit, 0, tr.maxUrls)
	for _, r := range config.Resources {
		log.Printf("checking: %s", r.Type)
		// Map the type to a fetchable URL (if applicable) or skip it if it's a non-fetchable type (primitive for example).
		urls, err := tr.MapFetchableURLs(r.Type)
		if err != nil {
			return nil, resolverError(config, fmt.Errorf("Failed to understand download url for %s: %v", r.Type, err))
		}
		if !existing[r.Type] {
			f := &fetchUnit{}
			for _, u := range urls {
				if len(u) > 0 {
					f.urls = append(f.urls, u)
					// Add to existing map so it is not fetched multiple times.
					existing[r.Type] = true
				}
			}
			if len(f.urls) > 0 {
				toFetch = append(toFetch, f)
				fetched[f.urls[0]] = append(fetched[f.urls[0]], &common.ImportFile{Name: r.Type, Path: f.urls[0]})
			}
		}
	}

	count := 0
	log.Printf("toFetch %#v", toFetch)
	for _, jj := range toFetch {
		log.Printf("TOFETCH UNIT: %#v", jj)
	}
	for len(toFetch) > 0 {
		log.Printf("toFetch2 %#v", toFetch)
		// 1. If short github URL, resolve to a download URL
		// 2. Fetch import URL. Exit if no URLs left
		// 3. Check/handle HTTP status
		// 4. Store results in all ImportFiles from that URL
		// 5. Check for the optional schema file at import URL + .schema
		// 6. Repeat 2,3 for schema file
		// 7. Add each schema import to fetch if not already done
		// 8. Mark URL done. Return to 1.
		if count >= tr.maxUrls {
			return nil, resolverError(config,
				fmt.Errorf("Number of imports exceeds maximum of %d", tr.maxUrls))
		}

		templates := []string{}
		url := toFetch[0].urls[0]
		for _, u := range toFetch[0].urls {
			template, err := performHTTPGet(tr.getter, u, false)
			if err != nil {
				return nil, resolverError(config, err)
			}
			templates = append(templates, template)
		}

		for _, i := range fetched[url] {
			template, err := parseContent(templates)
			if err != nil {
				return nil, resolverError(config, err)
			}
			i.Content = template
		}

		schemaURL := url + schemaSuffix
		sch, err := performHTTPGet(tr.getter, schemaURL, true)
		if err != nil {
			return nil, resolverError(config, err)
		}

		if sch != "" {
			var s common.Schema
			if err := yaml.Unmarshal([]byte(sch), &s); err != nil {
				return nil, resolverError(config, err)
			}
			// Here we handle any nested imports in the schema we've just fetched.
			for _, v := range s.Imports {
				i := &common.ImportFile{Name: v.Name}
				var existingSchema string
				urls, conversionErr := tr.MapFetchableURLs(v.Path)
				if conversionErr != nil {
					return nil, resolverError(config, fmt.Errorf("Failed to understand download url for %s: %v", v.Path, conversionErr))
				}
				if len(urls) == 0 {
					// If it's not a fetchable URL, we need to use the type name as is, since it is a short name
					// for a schema.
					urls = []string{v.Path}
				}
				for _, u := range urls {
					if len(fetched[u]) == 0 {
						// If this import URL is new to us, add it to the URLs to fetch.
						toFetch = append(toFetch, &fetchUnit{[]string{u}})
					} else {
						// If this is not a new import URL and we've already fetched its contents,
						// reuse them. Also, check if we also found a schema for that import URL and
						// record those contents for re-use as well.
						if fetched[u][0].Content != "" {
							i.Content = fetched[u][0].Content
							if len(fetched[u+schemaSuffix]) > 0 {
								existingSchema = fetched[u+schemaSuffix][0].Content
							}
						}
					}
					fetched[u] = append(fetched[u], i)
					if existingSchema != "" {
						fetched[u+schemaSuffix] = append(fetched[u+schemaSuffix],
							&common.ImportFile{Name: v.Name + schemaSuffix, Content: existingSchema})
					}
				}
			}

			// Add the schema we've fetched as the schema for any templates which used this URL.
			for _, i := range fetched[url] {
				schemaImportName := i.Name + schemaSuffix
				fetched[schemaURL] = append(fetched[schemaURL],
					&common.ImportFile{Name: schemaImportName, Content: sch})
			}
		}

		count = count + 1
		toFetch = toFetch[1:]
	}

	ret := []*common.ImportFile{}
	for _, v := range fetched {
		ret = append(ret, v...)
	}

	return ret, nil
}

// MapFetchableUrls checks a type to see if it is either a short git hub url or a fully specified URL
// and returns the URL that should be used to fetch it. If the url is not fetchable (primitive type for
// example) will return empty string.
func (tr *typeResolver) MapFetchableURLs(t string) ([]string, error) {
	if util.IsGithubShortType(t) {
		return tr.ShortTypeToDownloadURLs(t)
	} else if util.IsGithubShortPackageType(t) {
		return tr.ShortTypeToPackageDownloadURLs(t)
	} else if util.IsHttpUrl(t) {
		return []string{t}, nil
	}
	return []string{}, nil
}

// ShortTypeToDownloadURLs converts a github URL into downloadable URL from github.
// Input must be of the type and is assumed to have been validated before this call:
// github.com/owner/repo/qualifier/type:version
// for example:
// github.com/kubernetes/application-dm-templates/storage/redis:v1
func (tr *typeResolver) ShortTypeToDownloadURLs(template string) ([]string, error) {
	m := util.TemplateRegistryMatcher.FindStringSubmatch(template)
	if len(m) != 6 {
		return []string{}, fmt.Errorf("Failed to parse short github url: %s", template)
	}
	r := tr.rp.GetGithubRegistry(m[1], m[2])
	t := registry.Type{m[3], m[4], m[5]}
	return r.GetURLs(t)
}

// ShortTypeToPackageDownloadURLs converts a github URL into downloadable URLs from github.
// Input must be of the type and is assumed to have been validated before this call:
// github.com/owner/repo/type
// for example:
// github.com/helm/charts/cassandra
func (tr *typeResolver) ShortTypeToPackageDownloadURLs(template string) ([]string, error) {
	m := util.PackageRegistryMatcher.FindStringSubmatch(template)
	if len(m) != 4 {
		return []string{}, fmt.Errorf("Failed to parse short github url: %s", template)
	}
	r := tr.rp.GetGithubPackageRegistry(m[1], m[2])
	t := registry.Type{Name: m[3]}
	return r.GetURLs(t)
}

func parseContent(templates []string) (string, error) {
	if len(templates) == 1 {
		return templates[0], nil
	} else {
		// If there are multiple URLs that need to be fetched, that implies it's a package
		// of raw Kubernetes objects. We need to fetch them all as a unit and create a
		// template representing a package out of that below.
		fakeConfig := &common.Configuration{}
		for _, template := range templates {
			o, err := util.ParseKubernetesObject([]byte(template))
			if err != nil {
				return "", fmt.Errorf("not a kubernetes object: %+v", template)
			}
			// Looks like a native Kubernetes object, create a configuration out of it
			fakeConfig.Resources = append(fakeConfig.Resources, o)
		}
		marshalled, err := yaml.Marshal(fakeConfig)
		if err != nil {
			return "", fmt.Errorf("Failed to marshal: %+v", fakeConfig)
		}
		return string(marshalled), nil
	}
}
