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
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/util"

	//        "golang.org/x/net/context"
	//        "golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"

	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
)

// GCSRegistry implements the ObbectStorageRegistry interface and implements a
// Deployment Manager templates registry.
//
// A registry root must be a directory that contains all the available charts,
// one or two files per template.
// name-version.tgz
// name-version.prov
type GCSRegistry struct {
	name           string
	shortURL       string
	bucket         string
	format         common.RegistryFormat
	credentialName string

	httpClient *http.Client
	service    *storage.Service
}

// RE for GCS storage

// ChartFormatMatcher matches the chart name format
var ChartFormatMatcher = regexp.MustCompile("(.*)-(.*).tgz")

// URLFormatMatcher matches the GCS URL format (gs:).
var URLFormatMatcher = regexp.MustCompile("gs://(.*)")

// NewGCSRegistry creates a GCS registry.
func NewGCSRegistry(name, shortURL string, httpClient *http.Client, gcsService *storage.Service) (*GCSRegistry, error) {
	format := fmt.Sprintf("%s;%s", common.VersionedRegistry, common.OneLevelRegistry)
	trimmed := util.TrimURLScheme(shortURL)
	m := URLFormatMatcher.FindStringSubmatch(shortURL)
	if len(m) != 2 {
		return nil, fmt.Errorf("URL must be of the form gs://<bucket> was: %s", shortURL)
	}

	return &GCSRegistry{
			name:       name,
			shortURL:   trimmed,
			format:     common.RegistryFormat(format),
			httpClient: httpClient,
			service:    gcsService,
			bucket:     m[1],
		},
		nil
}

// GetRegistryName returns the name of the registry.
func (g GCSRegistry) GetRegistryName() string {
	return g.name
}

// GetBucket returns the registry bucket.
func (g GCSRegistry) GetBucket() string {
	return g.bucket
}

// GetRegistryType returns the registry type.
func (g GCSRegistry) GetRegistryType() common.RegistryType {
	return common.GCSRegistryType
}

// ListTypes lists types in this registry whose string values conform to the
// supplied regular expression, or all types, if the regular expression is nil.
func (g GCSRegistry) ListTypes(regex *regexp.Regexp) ([]Type, error) {
	// List all files in the bucket/prefix that contain the
	types := []Type{}

	// List all objects in a bucket using pagination
	pageToken := ""
	for {
		call := g.service.Objects.List(g.bucket)
		call.Delimiter("/")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		res, err := call.Do()
		if err != nil {
			return []Type{}, err
		}
		for _, object := range res.Items {
			// Charts should be named bucket/chart-X.Y.Z.tgz, so tease apart the version here
			m := ChartFormatMatcher.FindStringSubmatch(object.Name)
			if len(m) != 3 {
				continue
			}

			t, err := NewType("", m[1], m[2])
			if err != nil {
				return []Type{}, fmt.Errorf("can't create a type type at path %#v", err)
			}
			types = append(types, t)
		}
		if pageToken = res.NextPageToken; pageToken == "" {
			break
		}
	}
	return types, nil
}

// GetRegistryFormat returns the registry format.
func (g GCSRegistry) GetRegistryFormat() common.RegistryFormat {
	return common.CollectionRegistry
}

// GetRegistryShortURL returns the short URL for the registry.
func (g GCSRegistry) GetRegistryShortURL() string {
	return g.shortURL
}

// GetDownloadURLs fetches the download URLs for a given Chart
func (g GCSRegistry) GetDownloadURLs(t Type) ([]*url.URL, error) {
	call := g.service.Objects.List(g.bucket)
	call.Delimiter("/")
	call.Prefix(t.String())
	res, err := call.Do()
	ret := []*url.URL{}
	if err != nil {
		return ret, err
	}
	for _, object := range res.Items {
		log.Printf("Found: %s", object.Name)
		u, err := url.Parse(object.MediaLink)
		if err != nil {
			return nil, fmt.Errorf("cannot parse URL from %s: %s", object.MediaLink, err)
		}
		ret = append(ret, u)
	}
	return ret, err
}

// Do performs an HTTP operation on the receiver's httpClient.
func (g GCSRegistry) Do(req *http.Request) (resp *http.Response, err error) {
	return g.httpClient.Do(req)
}
