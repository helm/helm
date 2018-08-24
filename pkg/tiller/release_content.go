/*
Copyright The Helm Authors.

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

package tiller

import (
	"github.com/pkg/errors"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
)

// GetReleaseContent gets all of the stored information for the given release.
func (s *ReleaseServer) GetReleaseContent(req *hapi.GetReleaseContentRequest) (*release.Release, error) {
	if err := validateReleaseName(req.Name); err != nil {
		return nil, errors.Errorf("releaseContent: Release name is invalid: %s", req.Name)
	}

	if req.Version <= 0 {
		return s.Releases.Last(req.Name)
	}

	return s.Releases.Get(req.Name, req.Version)
}
