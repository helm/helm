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
	"regexp"
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
	re      *regexp.Regexp
	rp      registry.RegistryProvider
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
	ret.re = regexp.MustCompile("github.com/(.*)/(.*)/(.*)/(.*):(.*)")
	ret.rp = &registry.DefaultRegistryProvider{}
	return ret
}

func resolverError(c *common.Configuration, err error) error {
	return fmt.Errorf("cannot resolve types in configuration %s due to: \n%s\n",
		c, err)
}

func performHTTPGet(g util.HTTPClient, u string, allowMissing bool) (content string, err error) {
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
	toFetch := make([]string, 0, tr.maxUrls)
	for _, r := range config.Resources {
		// Map the type to a fetchable URL (if applicable) or skip it if it's a non-fetchable type (primitive for example).
		u, err := tr.MapFetchableURL(r.Type)
		if err != nil {
			return nil, resolverError(config, fmt.Errorf("Failed to understand download url for %s: %v", r.Type, err))
		}
		if len(u) > 0 && !existing[r.Type] {
			toFetch = append(toFetch, u)
			fetched[u] = append(fetched[u], &common.ImportFile{Name: r.Type, Path: u})
			// Add to existing map so it is not fetched multiple times.
			existing[r.Type] = true
		}
	}

	count := 0
	for len(toFetch) > 0 {
		//1. If short github URL, resolve to a download URL
		//2. Fetch import URL. Exit if no URLs left
		//3. Check/handle HTTP status
		//4. Store results in all ImportFiles from that URL
		//5. Check for the optional schema file at import URL + .schema
		//6. Repeat 2,3 for schema file
		//7. Add each schema import to fetch if not already done
		//8. Mark URL done. Return to 1.
		if count >= tr.maxUrls {
			return nil, resolverError(config,
				fmt.Errorf("Number of imports exceeds maximum of %d", tr.maxUrls))
		}

		url := toFetch[0]
		template, err := performHTTPGet(tr.getter, url, false)
		if err != nil {
			return nil, resolverError(config, err)
		}

		for _, i := range fetched[url] {
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
				u, conversionErr := tr.MapFetchableURL(v.Path)
				if conversionErr != nil {
					return nil, resolverError(config, fmt.Errorf("Failed to understand download url for %s: %v", v.Path, conversionErr))
				}
				// If it's not a fetchable URL, we need to use the type name as is, since it is a short name
				// for a schema.
				if len(u) == 0 {
					u = v.Path
				}
				if len(fetched[u]) == 0 {
					// If this import URL is new to us, add it to the URLs to fetch.
					toFetch = append(toFetch, u)
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

// MapFetchableUrl checks a type to see if it is either a short git hub url or a fully specified URL
// and returns the URL that should be used to fetch it. If the url is not fetchable (primitive type for
// example) will return empty string.
func (tr *typeResolver) MapFetchableURL(t string) (string, error) {
	if util.IsGithubShortType(t) {
		return tr.ShortTypeToDownloadURL(t)
	} else if util.IsHttpUrl(t) {
		return t, nil
	}
	return "", nil
}

// ShortTypeToDownloadURL converts a github URL into downloadable URL from github.
// Input must be of the type and is assumed to have been validated before this call:
// github.com/owner/repo/qualifier/type:version
// for example:
// github.com/kubernetes/application-dm-templates/storage/redis:v1
func (tr *typeResolver) ShortTypeToDownloadURL(template string) (string, error) {
	m := tr.re.FindStringSubmatch(template)
	if len(m) != 6 {
		return "", fmt.Errorf("Failed to parse short github url: %s", template)
	}
	r := tr.rp.GetGithubRegistry(m[1], m[2])
	t := registry.Type{m[3], m[4], m[5]}
	return r.GetURL(t)
}
