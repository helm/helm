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
	"github.com/kubernetes/helm/pkg/util"

	storage "google.golang.org/api/storage/v1"

	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// GCSRepoURLMatcher matches the GCS repository URL format (gs://<bucket>).
var GCSRepoURLMatcher = regexp.MustCompile("gs://(.*)")

// GCSChartURLMatcher matches the GCS chart URL format (gs://<bucket>/<name>-<version>.tgz).
var GCSChartURLMatcher = regexp.MustCompile("gs://(.*)/(.*)-(.*).tgz")

const (
	// GCSRepoType identifies the GCS repository type.
	GCSRepoType = ERepoType("gcs")

	// GCSRepoFormat identifies the GCS repository format.
	// In a GCS repository all charts appear at the top level.
	GCSRepoFormat = FlatRepoFormat

	// GCSPublicRepoBucket is the name of the public GCS repository bucket.
	GCSPublicRepoBucket = "kubernetes-charts"

	// GCSPublicRepoURL is the URL for the public GCS repository.
	GCSPublicRepoURL = "gs://" + GCSPublicRepoBucket
)

// GCSRepo implements the IStorageRepo interface for Google Cloud Storage.
type GCSRepo struct {
	Repo
	bucket     string
	httpClient *http.Client
	service    *storage.Service
}

// NewPublicGCSRepo creates a new an IStorageRepo for the public GCS repository.
func NewPublicGCSRepo(httpClient *http.Client) (*GCSRepo, error) {
	return NewGCSRepo(GCSPublicRepoURL, "", GCSPublicRepoBucket, nil)
}

// NewGCSRepo creates a new IStorageRepo for a given GCS repository.
func NewGCSRepo(URL, credentialName, repoName string, httpClient *http.Client) (*GCSRepo, error) {
	r, err := newRepo(URL, credentialName, repoName, GCSRepoFormat, GCSRepoType)
	if err != nil {
		return nil, err
	}

	return newGCSRepo(r, httpClient)
}

func newGCSRepo(r *Repo, httpClient *http.Client) (*GCSRepo, error) {
	URL := r.GetURL()
	m := GCSRepoURLMatcher.FindStringSubmatch(URL)
	if len(m) != 2 {
		return nil, fmt.Errorf("URL must be of the form gs://<bucket>, was %s", URL)
	}

	if err := validateRepoType(r.GetType()); err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	gcs, err := storage.New(httpClient)
	if err != nil {
		return nil, fmt.Errorf("cannot create storage service for %s: %s", URL, err)
	}

	gcsr := &GCSRepo{
		Repo:       *r,
		httpClient: httpClient,
		service:    gcs,
		bucket:     m[1],
	}

	return gcsr, nil
}

func validateRepoType(repoType ERepoType) error {
	switch repoType {
	case GCSRepoType:
		return nil
	}

	return fmt.Errorf("unknown repository type: %s", repoType)
}

// ListCharts lists charts in this chart repository whose string values conform to the
// supplied regular expression, or all charts, if the regular expression is nil.
func (g *GCSRepo) ListCharts(regex *regexp.Regexp) ([]string, error) {
	charts := []string{}

	// ListRepos all objects in a bucket using pagination
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
			// Charts should be named chart-X.Y.Z.tgz, so tease apart the name
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
	// Charts should be named chart-X.Y.Z.tgz, so check that the name matches
	if !ChartNameMatcher.MatchString(name) {
		return nil, fmt.Errorf("name must be of the form <name>-<version>.tgz, was %s", name)
	}

	call := g.service.Objects.Get(g.bucket, name)
	object, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("cannot get storage object named %s/%s: %s", g.bucket, name, err)
	}

	u, err := url.Parse(object.MediaLink)
	if err != nil {
		return nil, fmt.Errorf("cannot parse URL %s for chart %s/%s: %s",
			object.MediaLink, object.Bucket, object.Name, err)
	}

	getter := util.NewHTTPClient(3, g.httpClient, util.NewSleeper())
	body, code, err := getter.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("cannot fetch URL %s for chart %s/%s: %d %s",
			object.MediaLink, object.Bucket, object.Name, code, err)
	}

	return chart.LoadDataFromReader(strings.NewReader(body))
}

// GetBucket returns the repository bucket.
func (g *GCSRepo) GetBucket() string {
	return g.bucket
}

// Do performs an HTTP operation on the receiver's httpClient.
func (g *GCSRepo) Do(req *http.Request) (resp *http.Response, err error) {
	return g.httpClient.Do(req)
}
