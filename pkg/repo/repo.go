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
	"github.com/kubernetes/helm/pkg/common"

	"fmt"
	"net/url"
	"regexp"
)

// ChartRepo abstracts a place that holds charts, which can be
// used in a Deployment Manager configuration. There can be multiple
// ChartRepo implementations.
type ChartRepo interface {
	// GetRepoName returns the name of this ChartRepo.
	GetRepoName() string
	// GetRepoType returns the type of this repo.
	GetRepoType() common.RepoType
	// GetRepoURL returns the URL to the root of this ChartRepo.
	GetRepoURL() string
	// GetRepoFormat returns the format of this ChartRepo.
	GetRepoFormat() common.RepoFormat

	// ListCharts lists charts in this repository whose string values
	// conform to the supplied regular expression or all charts if regex is nil
	ListCharts(regex *regexp.Regexp) ([]string, error)
	// GetChart retrieves, unpacks and returns a chart by name.
	GetChart(name string) (*chart.Chart, error)
}

// ObjectStorageRepo abstracts a repository that resides in an Object Storage, for
// example Google Cloud Storage or AWS S3, etc.
type ObjectStorageRepo interface {
	ChartRepo // An ObjectStorageRepo is a ChartRepo
	GetBucket() string
}

type chartRepo struct {
	Name   string            `json:"name,omitempty"`   // The name of this ChartRepo
	URL    string            `json:"url,omitempty"`    // The URL to the root of this ChartRepo
	Format common.RepoFormat `json:"format,omitempty"` // The format of this ChartRepo
	Type   common.RepoType   `json:"type,omitempty"`   // The type of this ChartRepo
}

// ChartNameMatcher matches the chart name format
var ChartNameMatcher = regexp.MustCompile("(.*)-(.*).tgz")

func newRepo(name, URL, format, t string) (*chartRepo, error) {
	_, err := url.Parse(URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL (%s): %s", URL, err)
	}

	result := &chartRepo{
		Name:   name,
		URL:    URL,
		Format: common.RepoFormat(format),
		Type:   common.RepoType(t),
	}

	return result, nil
}

// GetRepoName returns the name of this ChartRepo.
func (cr *chartRepo) GetRepoName() string {
	return cr.Name
}

// GetRepoType returns the type of this repo.
func (cr *chartRepo) GetRepoType() common.RepoType {
	return cr.Type
}

// GetRepoURL returns the URL to the root of this ChartRepo.
func (cr *chartRepo) GetRepoURL() string {
	return cr.URL
}

// GetRepoFormat returns the format of this ChartRepo.
func (cr *chartRepo) GetRepoFormat() common.RepoFormat {
	return cr.Format
}
