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
	"reflect"
	"testing"

	"k8s.io/helm/pkg/helm"
	rpb "k8s.io/helm/pkg/proto/hapi/release"
	tpb "k8s.io/helm/pkg/proto/hapi/services"
)

func TestGetHistory_WithRevisions(t *testing.T) {
	mk := func(name string, vers int32, code rpb.Status_Code) *rpb.Release {
		return &rpb.Release{
			Name:    name,
			Version: vers,
			Info:    &rpb.Info{Status: &rpb.Status{Code: code}},
		}
	}

	// GetReleaseHistoryTests
	tests := []struct {
		desc string
		req  *tpb.GetHistoryRequest
		res  *tpb.GetHistoryResponse
	}{
		{
			desc: "get release with history and default limit (max=256)",
			req:  &tpb.GetHistoryRequest{Name: "angry-bird", Max: 256},
			res: &tpb.GetHistoryResponse{Releases: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
				mk("angry-bird", 2, rpb.Status_SUPERSEDED),
				mk("angry-bird", 1, rpb.Status_SUPERSEDED),
			}},
		},
		{
			desc: "get release with history using result limit (max=2)",
			req:  &tpb.GetHistoryRequest{Name: "angry-bird", Max: 2},
			res: &tpb.GetHistoryResponse{Releases: []*rpb.Release{
				mk("angry-bird", 4, rpb.Status_DEPLOYED),
				mk("angry-bird", 3, rpb.Status_SUPERSEDED),
			}},
		},
	}

	// test release history for release 'angry-bird'
	hist := []*rpb.Release{
		mk("angry-bird", 4, rpb.Status_DEPLOYED),
		mk("angry-bird", 3, rpb.Status_SUPERSEDED),
		mk("angry-bird", 2, rpb.Status_SUPERSEDED),
		mk("angry-bird", 1, rpb.Status_SUPERSEDED),
	}

	srv := rsFixture()
	for _, rls := range hist {
		if err := srv.env.Releases.Create(rls); err != nil {
			t.Fatalf("Failed to create release: %s", err)
		}
	}

	// run tests
	for _, tt := range tests {
		res, err := srv.GetHistory(helm.NewContext(), tt.req)
		if err != nil {
			t.Fatalf("%s:\nFailed to get History of %q: %s", tt.desc, tt.req.Name, err)
		}
		if !reflect.DeepEqual(res, tt.res) {
			t.Fatalf("%s:\nExpected:\n\t%+v\nActual\n\t%+v", tt.desc, tt.res, res)
		}
	}
}

func TestGetHistory_WithNoRevisions(t *testing.T) {
	tests := []struct {
		desc string
		req  *tpb.GetHistoryRequest
	}{
		{
			desc: "get release with no history",
			req:  &tpb.GetHistoryRequest{Name: "sad-panda", Max: 256},
		},
	}

	// create release 'sad-panda' with no revision history
	rls := namedReleaseStub("sad-panda", rpb.Status_DEPLOYED)
	srv := rsFixture()
	srv.env.Releases.Create(rls)

	for _, tt := range tests {
		res, err := srv.GetHistory(helm.NewContext(), tt.req)
		if err != nil {
			t.Fatalf("%s:\nFailed to get History of %q: %s", tt.desc, tt.req.Name, err)
		}
		if len(res.Releases) > 1 {
			t.Fatalf("%s:\nExpected zero items, got %d", tt.desc, len(res.Releases))
		}
	}
}
