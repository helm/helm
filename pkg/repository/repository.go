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

package repository

import (
	"github.com/kubernetes/helm/pkg/common"

	"regexp"
)

// Repository abstracts a repository that holds charts, which can be
// used in a Deployment Manager configuration. There can be multiple
// repository implementations.
type Repository interface {
	// GetRepositoryName returns the name of this repository
	GetRepositoryName() string
	// GetRepositoryChart returns the chart of this repository.
	GetRepositoryChart() common.RepositoryChart
	// GetRepositoryShortURL returns the short URL for this repository.
	GetRepositoryShortURL() string
	// GetRepositoryFormat returns the format of this repository.
	GetRepositoryFormat() common.RepositoryFormat

	// ListCharts lists charts in this repository whose string values conform to the
	// supplied regular expression, or all charts, if the regular expression is nil.
	ListCharts(regex *regexp.Regexp) ([]*common.Chart, error)
	GetChart(name string) (*common.Chart, error)
}

// ObjectStorageRepository abstracts a repository that resides in an Object Storage, for
// example Google Cloud Storage or AWS S3, etc.
type ObjectStorageRepository interface {
	Repository // An ObjectStorageRepository is a Repository.
	GetBucket() string
}
