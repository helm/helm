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
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"

	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

type repoProvider struct {
	sync.RWMutex
	rs    IRepoService
	cp    ICredentialProvider
	gcsrp IGCSRepoProvider
	repos map[string]IChartRepo
}

// NewRepoProvider creates a new repository provider.
func NewRepoProvider(rs IRepoService, gcsrp IGCSRepoProvider, cp ICredentialProvider) IRepoProvider {
	return newRepoProvider(rs, gcsrp, cp)
}

// newRepoProvider creates a new repository provider.
func newRepoProvider(rs IRepoService, gcsrp IGCSRepoProvider, cp ICredentialProvider) *repoProvider {
	if rs == nil {
		rs = NewInmemRepoService()
	}

	if cp == nil {
		cp = NewInmemCredentialProvider()
	}

	if gcsrp == nil {
		gcsrp = NewGCSRepoProvider(cp)
	}

	repos := make(map[string]IChartRepo)
	rp := &repoProvider{rs: rs, gcsrp: gcsrp, cp: cp, repos: repos}
	return rp
}

// GetRepoService returns the repository service used by this repository provider.
func (rp *repoProvider) GetRepoService() IRepoService {
	return rp.rs
}

// GetCredentialProvider returns the credential provider used by this repository provider.
func (rp *repoProvider) GetCredentialProvider() ICredentialProvider {
	return rp.cp
}

// GetGCSRepoProvider returns the GCS repository provider used by this repository provider.
func (rp *repoProvider) GetGCSRepoProvider() IGCSRepoProvider {
	return rp.gcsrp
}

// GetRepoByURL returns the repository with the given name.
func (rp *repoProvider) GetRepoByURL(URL string) (IChartRepo, error) {
	rp.Lock()
	defer rp.Unlock()

	if r, ok := rp.repos[URL]; ok {
		return r, nil
	}

	cr, err := rp.rs.GetRepoByURL(URL)
	if err != nil {
		return nil, err
	}

	return rp.createRepoByType(cr)
}

func (rp *repoProvider) createRepoByType(r IRepo) (IChartRepo, error) {
	switch r.GetType() {
	case GCSRepoType:
		cr, err := rp.gcsrp.GetGCSRepo(r)
		if err != nil {
			return nil, err
		}

		return rp.createRepo(cr)
	}

	return nil, fmt.Errorf("unknown repository type: %s", r.GetType())
}

func (rp *repoProvider) createRepo(cr IChartRepo) (IChartRepo, error) {
	URL := cr.GetURL()
	if _, ok := rp.repos[URL]; ok {
		return nil, fmt.Errorf("respository with URL %s already exists", URL)
	}

	rp.repos[URL] = cr
	return cr, nil
}

// GetRepoByChartURL returns the repository whose URL is a prefix of the given URL.
func (rp *repoProvider) GetRepoByChartURL(URL string) (IChartRepo, error) {
	rp.Lock()
	defer rp.Unlock()

	if r := rp.findRepoByChartURL(URL); r != nil {
		return r, nil
	}

	cr, err := rp.rs.GetRepoByChartURL(URL)
	if err != nil {
		return nil, err
	}

	return rp.createRepoByType(cr)
}

func (rp *repoProvider) findRepoByChartURL(URL string) IChartRepo {
	var found IChartRepo
	for _, r := range rp.repos {
		rURL := r.GetURL()
		if strings.HasPrefix(URL, rURL) {
			if found == nil || len(found.GetURL()) < len(rURL) {
				found = r
			}
		}
	}

	return found
}

// GetChartByReference maps the supplied chart reference into a fully qualified
// URL, uses the URL to find the repository it references, queries the repository
// for the chart, and then returns the chart and the repository that backs it.
func (rp *repoProvider) GetChartByReference(reference string) (*chart.Chart, IChartRepo, error) {
	l, URL, err := parseChartReference(reference)
	if err != nil {
		return nil, nil, err
	}

	r, err := rp.GetRepoByChartURL(URL)
	if err != nil {
		return nil, nil, err
	}

	name := fmt.Sprintf("%s-%s.tgz", l.Name, l.Version)
	c, err := r.GetChart(name)
	if err != nil {
		return nil, nil, err
	}

	return c, r, nil
}

// IsChartReference returns true if the supplied string is a reference to a chart in a repository
func IsChartReference(reference string) bool {
	if _, err := ParseChartReference(reference); err != nil {
		return false
	}

	return true
}

// ParseChartReference parses a reference to a chart in a repository and returns the URL for the chart
func ParseChartReference(reference string) (*chart.Locator, error) {
	l, _, err := parseChartReference(reference)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func parseChartReference(reference string) (*chart.Locator, string, error) {
	l, err := chart.Parse(reference)
	if err != nil {
		return nil, "", fmt.Errorf("cannot parse chart reference %s: %s", reference, err)
	}

	URL, err := l.Long(true)
	if err != nil {
		return nil, "", fmt.Errorf("chart reference %s does not resolve to a URL: %s", reference, err)
	}

	return l, URL, nil
}

type gcsRepoProvider struct {
	cp ICredentialProvider
}

// NewGCSRepoProvider creates a IGCSRepoProvider.
func NewGCSRepoProvider(cp ICredentialProvider) IGCSRepoProvider {
	if cp == nil {
		cp = NewInmemCredentialProvider()
	}

	return gcsRepoProvider{cp: cp}
}

// GetGCSRepo returns a new Google Cloud Storage repository. If a credential is specified, it will try to
// fetch it and use it, and if the credential isn't found, it will fall back to an unauthenticated client.
func (gcsrp gcsRepoProvider) GetGCSRepo(r IRepo) (IStorageRepo, error) {
	client, err := gcsrp.createGCSClient(r.GetCredentialName())
	if err != nil {
		return nil, err
	}

	return NewGCSRepo(r.GetURL(), r.GetCredentialName(), r.GetName(), client)
}

func (gcsrp gcsRepoProvider) createGCSClient(credentialName string) (*http.Client, error) {
	if credentialName == "" {
		return http.DefaultClient, nil
	}

	c, err := gcsrp.cp.GetCredential(credentialName)
	if err != nil {
		log.Printf("credential named %s not found: %s", credentialName, err)
		log.Print("falling back to the default client")
		return http.DefaultClient, nil
	}

	config, err := google.JWTConfigFromJSON([]byte(c.ServiceAccount), storage.DevstorageReadOnlyScope)
	if err != nil {
		log.Fatalf("cannot parse client secret file: %s", err)
	}

	return config.Client(oauth2.NoContext), nil
}

// IsGCSChartReference returns true if the supplied string is a reference to a chart in a GCS repository
func IsGCSChartReference(reference string) bool {
	if _, err := ParseGCSChartReference(reference); err != nil {
		return false
	}

	return true
}

// ParseGCSChartReference parses a reference to a chart in a GCS repository and returns the URL for the chart
func ParseGCSChartReference(reference string) (*chart.Locator, error) {
	l, URL, err := parseChartReference(reference)
	if err != nil {
		return nil, err
	}

	m := GCSChartURLMatcher.FindStringSubmatch(URL)
	if len(m) != 4 {
		return nil, fmt.Errorf("chart reference %s resolve to invalid URL: %s", reference, URL)
	}

	return l, nil
}
