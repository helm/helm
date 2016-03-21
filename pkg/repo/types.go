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

// RepoCredential holds a credential used to access a repository.
type RepoCredential struct {
	APIToken       APITokenCredential  `json:"apitoken,omitempty"`
	BasicAuth      BasicAuthCredential `json:"basicauth,omitempty"`
	ServiceAccount JWTTokenCredential  `json:"serviceaccount,omitempty"`
}

// CredentialProvider provides credentials for chart repositories.
type CredentialProvider interface {
	// SetCredential sets the credential for a repository.
	// May not be supported by some repository services.
	SetCredential(name string, credential *RepoCredential) error

	// GetCredential returns the specified credential or nil if there's no credential.
	// Error is non-nil if fetching the credential failed.
	GetCredential(name string) (*RepoCredential, error)
}

// RepoType defines the technology that implements a repository.
type RepoType string

// RepoFormat is a semi-colon delimited string that describes the format of a repository.
type RepoFormat string

const (
	// PathRepoFormat identfies a repository where charts are organized hierarchically.
	PathRepoFormat = RepoFormat("path")
	// FlatRepoFormat identifies a repository where all charts appear at the top level.
	FlatRepoFormat = RepoFormat("flat")
)

// Repo abstracts a repository.
type Repo interface {
	// GetName returns the friendly name of this repository.
	GetName() string
	// GetURL returns the URL to the root of this repository.
	GetURL() string
	// GetCredentialName returns the credential name used to access this repository.
	GetCredentialName() string
	// GetFormat returns the format of this repository.
	GetFormat() RepoFormat
	// GetType returns the technology implementing this repository.
	GetType() RepoType
}

// ChartRepo abstracts a place that holds charts.
type ChartRepo interface {
	// A ChartRepo is a Repo
	Repo

	// ListCharts lists charts in this repository whose string values
	// conform to the supplied regular expression, or all charts if regex is nil
	ListCharts(regex *regexp.Regexp) ([]string, error)

	// GetChart retrieves, unpacks and returns a chart by name.
	GetChart(name string) (*chart.Chart, error)
}

// ObjectStorageRepo abstracts a repository that resides in Object Storage,
// such as Google Cloud Storage, AWS S3, etc.
type ObjectStorageRepo interface {
	// An ObjectStorageRepo is a ChartRepo
	ChartRepo

	// GetBucket returns the name of the bucket that contains this repository.
	GetBucket() string
}

// RepoService maintains a list of chart repositories that defines the scope of all
// repository based operations, such as search and chart reference resolution.
type RepoService interface {
	// List returns the list of all known chart repositories
	List() ([]Repo, error)
	// Create adds a known repository to the list
	Create(repository Repo) error
	// Get returns the repository with the given name
	Get(name string) (Repo, error)
	// GetByURL returns the repository that backs the given URL
	GetByURL(URL string) (Repo, error)
	// Delete removes a known repository from the list
	Delete(name string) error
}
