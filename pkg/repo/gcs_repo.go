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

package repo

import (
	"github.com/kubernetes/helm/pkg/chart"
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/util"

	storage "google.golang.org/api/storage/v1"

	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

// GCSRepo implements the ObjectStorageRepo interface
// for Google Cloud Storage.
//
// A GCSRepo root must be a directory that contains all the available charts.
type GCSRepo struct {
	chartRepo      // A GCSRepo is a chartRepo
	bucket         string
	credentialName string
	httpClient     *http.Client
	service        *storage.Service
}

// URLFormatMatcher matches the GCS URL format (gs:).
var URLFormatMatcher = regexp.MustCompile("gs://(.*)")

var GCSRepoFormat = common.RepoFormat(fmt.Sprintf("%s;%s", common.UnversionedRepo, common.OneLevelRepo))

// NewGCSRepo creates a GCS repository.
func NewGCSRepo(name, URL string, httpClient *http.Client) (*GCSRepo, error) {
	m := URLFormatMatcher.FindStringSubmatch(URL)
	if len(m) != 2 {
		return nil, fmt.Errorf("URL must be of the form gs://<bucket>, was %s", URL)
	}

	cr, err := newRepo(name, URL, string(GCSRepoFormat), string(common.GCSRepoType))
	if err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	gs, err := storage.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("cannot create storage service for %s: %s", URL, err)
	}

	result := &GCSRepo{
		chartRepo:  *cr,
		httpClient: httpClient,
		service:    gs,
		bucket:     m[1],
	}

	return result, nil
}

// GetBucket returns the repository bucket.
func (g *GCSRepo) GetBucket() string {
	return g.bucket
}

// ListCharts lists charts in this chart repository whose string values conform to the
// supplied regular expression, or all charts, if the regular expression is nil.
func (g *GCSRepo) ListCharts(regex *regexp.Regexp) ([]string, error) {
	// List all files in the bucket/prefix that contain the
	charts := []string{}

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
			return nil, err
		}
		for _, object := range res.Items {
			// Charts should be named bucket/chart-X.Y.Z.tgz, so tease apart the version here
			m := ChartNameMatcher.FindStringSubmatch(object.Name)
			if len(m) != 3 {
				continue
			}

			if regex == nil || regex.MatchString(object.Name) {
				charts = append(charts, object.Name)
			}
		}

		if pageToken = res.NextPageToken; pageToken == "" {
			break
		}
	}

	return charts, nil
}

// GetChart retrieves, unpacks and returns a chart by name.
func (g *GCSRepo) GetChart(name string) (*chart.Chart, error) {
	// Charts should be named bucket/chart-X.Y.Z.tgz, so tease apart the version here
	if !ChartNameMatcher.MatchString(name) {
		return nil, fmt.Errorf("name must be of the form <name>-<version>.tgz, was %s", name)
	}

	call := g.service.Objects.Get(g.bucket, name)
	object, err := call.Do()
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(object.MediaLink)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse URL %s for chart %s/%s: %s",
			object.MediaLink, object.Bucket, object.Name, err)
	}

	getter := util.NewHTTPClient(3, g.httpClient, util.NewSleeper())
	body, code, err := getter.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("Cannot fetch URL %s for chart %s/%s: %d %s",
			object.MediaLink, object.Bucket, object.Name, code, err)
	}

	return chart.Load(body)
}

// Do performs an HTTP operation on the receiver's httpClient.
func (g *GCSRepo) Do(req *http.Request) (resp *http.Response, err error) {
	return g.httpClient.Do(req)
}

// TODO: Remove GetShortURL when no longer needed.

// GetShortURL returns the URL without the scheme.
func (g GCSRepo) GetShortURL() string {
	return util.TrimURLScheme(g.URL)
}
