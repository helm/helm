/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, version 2.0 (the "License");
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

	"regexp"
)

// ChartNameMatcher matches the chart name format
var ChartNameMatcher = regexp.MustCompile("(.*)-(.*).tgz")

// BasicAuthCredential holds a username and password.
type BasicAuthCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// APITokenCredential defines an API token.
type APITokenCredential string

// JWTTokenCredential defines a JWT token.
type JWTTokenCredential string

// Credential holds a credential used to access a repository.
type Credential struct {
	APIToken       APITokenCredential  `json:"apitoken,omitempty"`
	BasicAuth      BasicAuthCredential `json:"basicauth,omitempty"`
	ServiceAccount JWTTokenCredential  `json:"serviceaccount,omitempty"`
}

// ICredentialProvider provides credentials for chart repositories.
type ICredentialProvider interface {
	// SetCredential sets the credential for a repository.
	// May not be supported by some repository services.
	SetCredential(name string, credential *Credential) error

	// GetCredential returns the specified credential or nil if there's no credential.
	// Error is non-nil if fetching the credential failed.
	GetCredential(name string) (*Credential, error)
}

// ERepoType defines the technology that implements a repository.
type ERepoType string

// ERepoFormat is a semi-colon delimited string that describes the format of a repository.
type ERepoFormat string

const (
	// PathRepoFormat identfies a repository where charts are organized hierarchically.
	PathRepoFormat = ERepoFormat("path")
	// FlatRepoFormat identifies a repository where all charts appear at the top level.
	FlatRepoFormat = ERepoFormat("flat")
)

// Repo describes a repository
type Repo struct {
	Name           string      `json:"name"`                     // Name of repository
	URL            string      `json:"url"`                      // URL to the root of this repository
	CredentialName string      `json:"credentialname,omitempty"` // Credential name used to access this repository
	Format         ERepoFormat `json:"format,omitempty"`         // Format of this repository
	Type           ERepoType   `json:"type,omitempty"`           // Technology implementing this repository
}

// IRepo abstracts a repository.
type IRepo interface {
	// GetName returns the name of the repository
	GetName() string
	// GetURL returns the URL to the root of this repository.
	GetURL() string
	// GetCredentialName returns the credential name used to access this repository.
	GetCredentialName() string
	// GetFormat returns the format of this repository.
	GetFormat() ERepoFormat
	// GetType returns the technology implementing this repository.
	GetType() ERepoType
}

// IChartRepo abstracts a place that holds charts.
type IChartRepo interface {
	// A IChartRepo is a IRepo
	IRepo

	// ListCharts lists the URLs for charts in this repository that
	// conform to the supplied regular expression, or all charts if regex is nil
	ListCharts(regex *regexp.Regexp) ([]string, error)

	// GetChart retrieves, unpacks and returns a chart by name.
	GetChart(name string) (*chart.Chart, error)
}

// IStorageRepo abstracts a repository that resides in Object Storage,
// such as Google Cloud Storage, AWS S3, etc.
type IStorageRepo interface {
	// An IStorageRepo is a IChartRepo
	IChartRepo

	// GetBucket returns the name of the bucket that contains this repository.
	GetBucket() string
}

// IRepoService maintains a list of chart repositories that defines the scope of all
// repository based operations, such as search and chart reference resolution.
type IRepoService interface {
	// ListRepos returns the list of all known chart repositories
	ListRepos() (map[string]string, error)
	// CreateRepo adds a known repository to the list
	CreateRepo(repository IRepo) error
	// GetRepoByURL returns the repository with the given name
	GetRepoByURL(name string) (IRepo, error)
	// GetRepoByChartURL returns the repository that backs the given URL
	GetRepoByChartURL(URL string) (IRepo, error)
	// DeleteRepo removes a known repository from the list
	DeleteRepo(name string) error
}

// IRepoProvider is a factory for IChartRepo instances.
type IRepoProvider interface {
	GetChartByReference(reference string) (*chart.Chart, IChartRepo, error)
	GetRepoByChartURL(URL string) (IChartRepo, error)
	GetRepoByURL(URL string) (IChartRepo, error)
}

// IGCSRepoProvider is a factory for GCS IRepo instances.
type IGCSRepoProvider interface {
	GetGCSRepo(r IRepo) (IStorageRepo, error)
}
