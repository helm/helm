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
	"bytes"

	"github.com/pkg/errors"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
)

// GetReleaseStatus gets the status information for a named release.
func (s *ReleaseServer) GetReleaseStatus(req *hapi.GetReleaseStatusRequest) (*hapi.GetReleaseStatusResponse, error) {
	if err := validateReleaseName(req.Name); err != nil {
		return nil, errors.Errorf("getStatus: Release name is invalid: %s", req.Name)
	}

	var rel *release.Release

	if req.Version <= 0 {
		var err error
		rel, err = s.Releases.Last(req.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "getting deployed release %q", req.Name)
		}
	} else {
		var err error
		if rel, err = s.Releases.Get(req.Name, req.Version); err != nil {
			return nil, errors.Wrapf(err, "getting release '%s' (v%d)", req.Name, req.Version)
		}
	}

	if rel.Info == nil {
		return nil, errors.New("release info is missing")
	}
	if rel.Chart == nil {
		return nil, errors.New("release chart is missing")
	}

	sc := rel.Info.Status
	statusResp := &hapi.GetReleaseStatusResponse{
		Name:      rel.Name,
		Namespace: rel.Namespace,
		Info:      rel.Info,
	}

	// Ok, we got the status of the release as we had jotted down, now we need to match the
	// manifest we stashed away with reality from the cluster.
	resp, err := s.KubeClient.Get(rel.Namespace, bytes.NewBufferString(rel.Manifest))
	if sc == release.StatusUninstalled || sc == release.StatusFailed {
		// Skip errors if this is already deleted or failed.
		return statusResp, nil
	} else if err != nil {
		return nil, errors.Wrapf(err, "warning: Get for %s failed", rel.Name)
	}
	rel.Info.Resources = resp
	return statusResp, nil
}
