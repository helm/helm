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
	"net/http"

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
	maxUrls int
	rp      registry.RegistryProvider
	c       util.HTTPClient
}

type fetchableURL struct {
	reg registry.Registry
	url string
}

type fetchUnit struct {
	urls []fetchableURL
}

// NewTypeResolver returns a new initialized TypeResolver.
func NewTypeResolver(rp registry.RegistryProvider, c util.HTTPClient) TypeResolver {
	return &typeResolver{maxUrls: maxURLImports, rp: rp, c: c}
}

func resolverError(c *common.Configuration, err error) error {
	return fmt.Errorf("cannot resolve types in configuration %s due to: \n%s\n",
		c, err)
}

func (tr *typeResolver) performHTTPGet(d util.HTTPDoer, u string, allowMissing bool) (content string, err error) {
	g := tr.c
	if d != nil {
		g = util.NewHTTPClient(3, d, util.NewSleeper())
	}
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
		// Map the type to a fetchable URL (if applicable) or skip it if it's a non-fetchable type (primitive for example).
		urls, urlRegistry, err := registry.GetDownloadURLs(tr.rp, r.Type)
		if err != nil {
			return nil, resolverError(config, fmt.Errorf("Failed to understand download url for %s: %v", r.Type, err))
		}
		if !existing[r.Type] {
			// Add to existing map so it is not fetched multiple times.
			existing[r.Type] = true
			if len(urls) > 0 {
				f := &fetchUnit{}
				for _, u := range urls {
					if len(u) > 0 {
						f.urls = append(f.urls, fetchableURL{urlRegistry, u})
					}
				}
				if len(f.urls) > 0 {
					toFetch = append(toFetch, f)
					fetched[f.urls[0].url] = append(fetched[f.urls[0].url], &common.ImportFile{Name: r.Type, Path: f.urls[0].url})
				}
			}
		}
	}

	count := 0
	for len(toFetch) > 0 {
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
			template, err := tr.performHTTPGet(u.reg, u.url, false)
			if err != nil {
				return nil, resolverError(config, err)
			}
			templates = append(templates, template)
		}

		for _, i := range fetched[url.url] {
			template, err := parseContent(templates)
			if err != nil {
				return nil, resolverError(config, err)
			}
			i.Content = template
		}

		schemaURL := url.url + schemaSuffix
		sch, err := tr.performHTTPGet(url.reg, schemaURL, true)
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
				urls, urlRegistry, conversionErr := registry.GetDownloadURLs(tr.rp, v.Path)
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
						toFetch = append(toFetch, &fetchUnit{[]fetchableURL{fetchableURL{urlRegistry, u}}})
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
			for _, i := range fetched[url.url] {
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

func parseContent(templates []string) (string, error) {
	// Try to parse the content as a slice of Kubernetes objects
	kos, err := parseKubernetesObjects(templates)
	if err == nil {
		return kos, nil
	}

	// If there's only one template, return it without asking questions
	if len(templates) == 1 {
		return templates[0], nil
	}

	return "", fmt.Errorf("cannot parse content: %v", templates)
}

// parseKubernetesObjects tries to parse the content as a package of raw Kubernetes objects.
// If the content parses, it returns a configuration representing the package containing one
// resource per object.
func parseKubernetesObjects(templates []string) (string, error) {
	fakeConfig := &common.Configuration{}
	for _, template := range templates {
		o, err := util.ParseKubernetesObject([]byte(template))
		if err != nil {
			return "", fmt.Errorf("not a kubernetes object: %+v", template)
		}
		// Looks like a native Kubernetes object, so add a resource that wraps it to the result.
		fakeConfig.Resources = append(fakeConfig.Resources, o)
	}
	marshalled, err := yaml.Marshal(fakeConfig)
	if err != nil {
		return "", fmt.Errorf("Failed to marshal: %+v", fakeConfig)
	}
	return string(marshalled), nil
}
