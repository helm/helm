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
	"time"

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
	ResolveTypes(config *Configuration, imports []*ImportFile) ([]*ImportFile, error)
}

type typeResolver struct {
	getter  util.HTTPClient
	maxUrls int
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
	return ret
}

func resolverError(c *Configuration, err error) error {
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
func (tr *typeResolver) ResolveTypes(config *Configuration, imports []*ImportFile) ([]*ImportFile, error) {
	existing := map[string]bool{}
	for _, v := range imports {
		existing[v.Name] = true
	}

	fetched := map[string][]*ImportFile{}
	toFetch := make([]string, 0, tr.maxUrls)
	for _, r := range config.Resources {
		// Only fetch HTTP URLs that we haven't already imported.
		if util.IsHttpUrl(r.Type) && !existing[r.Type] {
			toFetch = append(toFetch, r.Type)

			fetched[r.Type] = append(fetched[r.Type], &ImportFile{Name: r.Type})

			// Add to existing map so it is not fetched multiple times.
			existing[r.Type] = true
		}
	}

	count := 0
	for len(toFetch) > 0 {
		//1. Fetch import URL. Exit if no URLs left
		//2. Check/handle HTTP status
		//3. Store results in all ImportFiles from that URL
		//4. Check for the optional schema file at import URL + .schema
		//5. Repeat 2,3 for schema file
		//6. Add each schema import to fetch if not already done
		//7. Mark URL done. Return to 1.
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
			var s Schema
			if err := yaml.Unmarshal([]byte(sch), &s); err != nil {
				return nil, resolverError(config, err)
			}
			// Here we handle any nested imports in the schema we've just fetched.
			for _, v := range s.Imports {
				i := &ImportFile{Name: v.Name}
				var existingSchema string
				if len(fetched[v.Path]) == 0 {
					// If this import URL is new to us, add it to the URLs to fetch.
					toFetch = append(toFetch, v.Path)
				} else {
					// If this is not a new import URL and we've already fetched its contents,
					// reuse them. Also, check if we also found a schema for that import URL and
					// record those contents for re-use as well.
					if fetched[v.Path][0].Content != "" {
						i.Content = fetched[v.Path][0].Content
						if len(fetched[v.Path+schemaSuffix]) > 0 {
							existingSchema = fetched[v.Path+schemaSuffix][0].Content
						}
					}
				}
				fetched[v.Path] = append(fetched[v.Path], i)
				if existingSchema != "" {
					fetched[v.Path+schemaSuffix] = append(fetched[v.Path+schemaSuffix],
						&ImportFile{Name: v.Name + schemaSuffix, Content: existingSchema})
				}
			}

			// Add the schema we've fetched as the schema for any templates which used this URL.
			for _, i := range fetched[url] {
				schemaImportName := i.Name + schemaSuffix
				fetched[schemaURL] = append(fetched[schemaURL],
					&ImportFile{Name: schemaImportName, Content: sch})
			}
		}

		count = count + 1
		toFetch = toFetch[1:]
	}

	ret := []*ImportFile{}
	for _, v := range fetched {
		ret = append(ret, v...)
	}

	return ret, nil
}
