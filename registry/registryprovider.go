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
	"github.com/kubernetes/deployment-manager/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	storage "google.golang.org/api/storage/v1"

	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// RegistryProvider is a factory for Registry instances.
type RegistryProvider interface {
	GetRegistryByShortURL(URL string) (Registry, error)
	GetRegistryByName(registryName string) (Registry, error)
}

type registryProvider struct {
	sync.RWMutex
	rs         common.RegistryService
	grp        GithubRegistryProvider
	gcsrp      GCSRegistryProvider
	cp         common.CredentialProvider
	registries map[string]Registry
}

func NewDefaultRegistryProvider(cp common.CredentialProvider, rs common.RegistryService) RegistryProvider {
	return NewRegistryProvider(rs, NewGithubRegistryProvider(cp), NewGCSRegistryProvider(cp), cp)
}

func NewRegistryProvider(rs common.RegistryService, grp GithubRegistryProvider, gcsrp GCSRegistryProvider, cp common.CredentialProvider) RegistryProvider {
	if rs == nil {
		rs = NewInmemRegistryService()
	}

	if cp == nil {
		cp = NewInmemCredentialProvider()
	}

	if grp == nil {
		grp = NewGithubRegistryProvider(cp)
	}

	if gcsrp == nil {
		gcsrp = NewGCSRegistryProvider(cp)
	}

	registries := make(map[string]Registry)
	rp := &registryProvider{rs: rs, grp: grp, gcsrp: gcsrp, cp: cp, registries: registries}
	return rp
}

func (rp *registryProvider) getRegistry(cr common.Registry) (Registry, error) {
	switch cr.Type {
	case common.GithubRegistryType:
		return rp.grp.GetGithubRegistry(cr)
	case common.GCSRegistryType:
		log.Printf("Creating a bigstore client using %#v", rp.gcsrp)
		return rp.gcsrp.GetGCSRegistry(cr)
	default:
		return nil, fmt.Errorf("unknown registry type: %s", cr.Type)
	}
}

func (rp *registryProvider) GetRegistryByShortURL(URL string) (Registry, error) {
	rp.RLock()
	defer rp.RUnlock()

	result := rp.findRegistryByShortURL(URL)
	if result == nil {
		cr, err := rp.rs.GetByURL(URL)
		if err != nil {
			return nil, err
		}

		r, err := rp.getRegistry(*cr)
		if err != nil {
			return nil, err
		}

		rp.registries[r.GetRegistryName()] = r
		result = r
	}

	return result, nil
}

// findRegistryByShortURL trims the scheme from both the supplied URL
// and the short URL returned by GetRegistryShortURL.
func (rp *registryProvider) findRegistryByShortURL(URL string) Registry {
	trimmed := util.TrimURLScheme(URL)
	for _, r := range rp.registries {
		if strings.HasPrefix(trimmed, util.TrimURLScheme(r.GetRegistryShortURL())) {
			return r
		}
	}

	return nil
}

func (rp *registryProvider) GetRegistryByName(registryName string) (Registry, error) {
	rp.RLock()
	defer rp.RUnlock()

	cr, err := rp.rs.Get(registryName)
	if err != nil {
		return nil, err
	}

	r, err := rp.getRegistry(*cr)
	if err != nil {
		return nil, err
	}

	rp.registries[r.GetRegistryName()] = r

	return r, nil
}

func ParseRegistryFormat(rf common.RegistryFormat) map[common.RegistryFormat]bool {
	split := strings.Split(string(rf), ";")
	var result = map[common.RegistryFormat]bool{}
	for _, format := range split {
		result[common.RegistryFormat(format)] = true
	}

	return result
}

// GithubRegistryProvider is a factory for GithubRegistry instances.
type GithubRegistryProvider interface {
	GetGithubRegistry(cr common.Registry) (GithubRegistry, error)
}

type githubRegistryProvider struct {
	cp common.CredentialProvider
}

// NewGithubRegistryProvider creates a GithubRegistryProvider.
func NewGithubRegistryProvider(cp common.CredentialProvider) GithubRegistryProvider {
	if cp == nil {
		panic(fmt.Errorf("no credential provider"))
	}
	return &githubRegistryProvider{cp: cp}
}

func (grp githubRegistryProvider) createGithubClient(credentialName string) (*http.Client, *github.Client, error) {
	if credentialName == "" {
		return http.DefaultClient, github.NewClient(nil), nil
	}
	c, err := grp.cp.GetCredential(credentialName)

	if err != nil {
		log.Printf("Failed to fetch credential %s: %v", credentialName, err)
		log.Print("Trying to use unauthenticated client")
		return http.DefaultClient, github.NewClient(nil), nil
	}

	if c != nil {
		if c.APIToken != "" {
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: string(c.APIToken)},
			)
			tc := oauth2.NewClient(oauth2.NoContext, ts)
			return tc, github.NewClient(tc), nil
		}
		if c.BasicAuth.Username != "" && c.BasicAuth.Password != "" {
			tp := github.BasicAuthTransport{
				Username: c.BasicAuth.Username,
				Password: c.BasicAuth.Password,
			}
			return tp.Client(), github.NewClient(tp.Client()), nil
		}

	}
	return nil, nil, fmt.Errorf("No suitable credential found for %s", credentialName)

}

// GetGithubRegistry returns a new GithubRegistry. If there's a credential that is specified, will try
// to fetch it and use it, and if there's no credential found, will fall back to unauthenticated client.
func (grp githubRegistryProvider) GetGithubRegistry(cr common.Registry) (GithubRegistry, error) {
	// If there's a credential that we need to use, fetch it and create a client for it.
	httpClient, client, err := grp.createGithubClient(cr.CredentialName)
	if err != nil {
		return nil, err
	}

	fMap := ParseRegistryFormat(cr.Format)
	if fMap[common.UnversionedRegistry] && fMap[common.OneLevelRegistry] {
		return NewGithubPackageRegistry(cr.Name, cr.URL, nil, httpClient, client)
	}

	if fMap[common.VersionedRegistry] && fMap[common.CollectionRegistry] {
		return NewGithubTemplateRegistry(cr.Name, cr.URL, nil, httpClient, client)
	}

	return nil, fmt.Errorf("unknown registry format: %s", cr.Format)
}

// GCSRegistryProvider is a factory for GCS Registry instances.
type GCSRegistryProvider interface {
	GetGCSRegistry(cr common.Registry) (ObjectStorageRegistry, error)
}

type gcsRegistryProvider struct {
	cp common.CredentialProvider
}

// NewGCSRegistryProvider creates a GCSRegistryProvider.
func NewGCSRegistryProvider(cp common.CredentialProvider) GCSRegistryProvider {
	if cp == nil {
		panic(fmt.Errorf("no credential provider"))
	}
	return &gcsRegistryProvider{cp: cp}
}

// GetGCSRegistry returns a new Google Cloud Storage . If there's a credential that is specified, will try
// to fetch it and use it, and if there's no credential found, will fall back to unauthenticated client.
func (gcsrp gcsRegistryProvider) GetGCSRegistry(cr common.Registry) (ObjectStorageRegistry, error) {
	// If there's a credential that we need to use, fetch it and create a client for it.
	if cr.CredentialName == "" {
		return nil, fmt.Errorf("No CredentialName specified for %s", cr.Name)
	}
	client, err := gcsrp.createGCSClient(cr.CredentialName)
	if err != nil {
		return nil, err
	}
	service, err := storage.New(client)
	if err != nil {
		log.Fatalf("Unable to create storage service: %v", err)
	}
	return NewGCSRegistry(cr.Name, cr.URL, client, service)
}

func (gcsrp gcsRegistryProvider) createGCSClient(credentialName string) (*http.Client, error) {
	c, err := gcsrp.cp.GetCredential(credentialName)
	if err != nil {
		log.Printf("Failed to fetch credential %s: %v", credentialName, err)
		return nil, fmt.Errorf("Failed to fetch Credential %s: %s", credentialName, err)
	}

	config, err := google.JWTConfigFromJSON([]byte(c.ServiceAccount), storage.DevstorageReadOnlyScope)

	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	return config.Client(oauth2.NoContext), nil
}

// RE for a registry type that does support versions and has collections.
var TemplateRegistryMatcher = regexp.MustCompile("github.com/(.*)/(.*)/(.*)/(.*):(.*)")

// RE for a registry type that does not support versions and does not have collections.
var PackageRegistryMatcher = regexp.MustCompile("github.com/(.*)/(.*)/(.*)")

// RE for GCS storage
var GCSRegistryMatcher = regexp.MustCompile("gs://(.*)/(.*)")

// IsGithubShortType returns whether a given type is a type description in a short format to a github repository type.
// For now, this means using github types:
// github.com/owner/repo/qualifier/type:version
// for example:
// github.com/kubernetes/application-dm-templates/storage/redis:v1
func IsGithubShortType(t string) bool {
	return TemplateRegistryMatcher.MatchString(t)
}

// IsGithubShortPackageType returns whether a given type is a type description in a short format to a github
// package repository type.
// For now, this means using github types:
// github.com/owner/repo/type
// for example:
// github.com/helm/charts/cassandra
func IsGithubShortPackageType(t string) bool {
	return PackageRegistryMatcher.MatchString(t)
}

// IsGCSShortType returns whether a given type is a type description in a short format to GCS
func IsGCSShortType(t string) bool {
	return strings.HasPrefix(t, "gs://")
}

// GetDownloadURLs checks a type to see if it is either a short git hub url or a fully specified URL
// and returns the URLs that should be used to fetch it. If the url is not fetchable (primitive type
// for example), it returns an empty slice.
func GetDownloadURLs(rp RegistryProvider, t string) ([]string, Registry, error) {
	if IsGithubShortType(t) {
		return ShortTypeToDownloadURLs(rp, t)
	} else if IsGithubShortPackageType(t) {
		return ShortTypeToPackageDownloadURLs(rp, t)
	} else if IsGCSShortType(t) {
		return ShortTypeToGCSDownloadUrls(rp, t)
	} else if util.IsHttpUrl(t) {
		result, err := url.Parse(t)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot parse download URL %s: %s", t, err)
		}

		return []string{result.String()}, nil, nil
	}

	return []string{}, nil, nil
}

// ShortTypeToDownloadURLs converts a github URL into downloadable URL from github.
// Input must be of the type and is assumed to have been validated before this call:
// github.com/owner/repo/qualifier/type:version
// for example:
// github.com/kubernetes/application-dm-templates/storage/redis:v1
func ShortTypeToDownloadURLs(rp RegistryProvider, t string) ([]string, Registry, error) {
	m := TemplateRegistryMatcher.FindStringSubmatch(t)
	if len(m) != 6 {
		return nil, nil, fmt.Errorf("cannot parse short github url: %s", t)
	}

	r, err := rp.GetRegistryByShortURL(t)
	if err != nil {
		return nil, nil, err
	}

	if r == nil {
		panic(fmt.Errorf("cannot get github registry for %s", t))
	}

	tt, err := NewType(m[3], m[4], m[5])
	if err != nil {
		return nil, r, err
	}

	urls, err := r.GetDownloadURLs(tt)
	if err != nil {
		return nil, r, err
	}

	return util.ConvertURLsToStrings(urls), r, err
}

// ShortTypeToPackageDownloadURLs converts a github URL into downloadable URLs from github.
// Input must be of the type and is assumed to have been validated before this call:
// github.com/owner/repo/type
// for example:
// github.com/helm/charts/cassandra
func ShortTypeToPackageDownloadURLs(rp RegistryProvider, t string) ([]string, Registry, error) {
	m := PackageRegistryMatcher.FindStringSubmatch(t)
	if len(m) != 4 {
		return nil, nil, fmt.Errorf("Failed to parse short github url: %s", t)
	}

	r, err := rp.GetRegistryByShortURL(t)
	if err != nil {
		return nil, nil, err
	}

	tt, err := NewType("", m[3], "")
	if err != nil {
		return nil, r, err
	}

	urls, err := r.GetDownloadURLs(tt)
	if err != nil {
		return nil, r, err
	}

	return util.ConvertURLsToStrings(urls), r, err
}

func ShortTypeToGCSDownloadUrls(rp RegistryProvider, t string) ([]string, Registry, error) {
	m := GCSRegistryMatcher.FindStringSubmatch(t)
	if len(m) != 3 {
		return nil, nil, fmt.Errorf("Failed to parse short gs url: %s", t)
	}

	r, err := rp.GetRegistryByShortURL(t)
	if err != nil {
		return nil, nil, err
	}
	tt, err := NewType(m[1], m[2], "")
	if err != nil {
		return nil, r, err
	}

	urls, err := r.GetDownloadURLs(tt)
	if err != nil {
		return nil, r, err
	}
	return util.ConvertURLsToStrings(urls), r, err
}
