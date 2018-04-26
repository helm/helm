/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"regexp"

	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
	relutil "k8s.io/helm/pkg/releaseutil"
)

// ListReleases lists the releases found by the server.
func (s *ReleaseServer) ListReleases(req *hapi.ListReleasesRequest) ([]*release.Release, error) {
	if len(req.StatusCodes) == 0 {
		req.StatusCodes = []release.ReleaseStatus{release.StatusDeployed}
	}

	rels, err := s.Releases.ListFilterAll(func(r *release.Release) bool {
		for _, sc := range req.StatusCodes {
			if sc == r.Info.Status {
				return true
			}
		}
		return false
	})
	if err != nil {
		return nil, err
	}

	if len(req.Filter) != 0 {
		rels, err = filterReleases(req.Filter, rels)
		if err != nil {
			return nil, err
		}
	}

	switch req.SortBy {
	case hapi.ListSortName:
		relutil.SortByName(rels)
	case hapi.ListSortLastReleased:
		relutil.SortByDate(rels)
	}

	if req.SortOrder == hapi.ListSortDesc {
		ll := len(rels)
		rr := make([]*release.Release, ll)
		for i, item := range rels {
			rr[ll-i-1] = item
		}
		rels = rr
	}

	return rels, nil
}

func filterReleases(filter string, rels []*release.Release) ([]*release.Release, error) {
	preg, err := regexp.Compile(filter)
	if err != nil {
		return rels, err
	}
	matches := []*release.Release{}
	for _, r := range rels {
		if preg.MatchString(r.Name) {
			matches = append(matches, r)
		}
	}
	return matches, nil
}
