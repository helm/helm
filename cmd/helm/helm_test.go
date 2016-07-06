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

package main

import (
	"github.com/golang/protobuf/ptypes/timestamp"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

func releaseMock(name string) *release.Release {
	date := timestamp.Timestamp{Seconds: 242085845, Nanos: 0}
	return &release.Release{
		Name: name,
		Info: &release.Info{
			FirstDeployed: &date,
			LastDeployed:  &date,
			Status:        &release.Status{Code: release.Status_DEPLOYED},
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "foo",
				Version: "0.1.0-beta.1",
			},
			Templates: []*chart.Template{
				{Name: "foo.tpl", Data: []byte("Hello")},
			},
		},
		Config:  &chart.Config{Raw: `name: "value"`},
		Version: 1,
	}
}

type fakeReleaseClient struct {
	rels []*release.Release
	err  error
}

func (c *fakeReleaseClient) ListReleases(opts ...helm.ReleaseListOption) (*rls.ListReleasesResponse, error) {
	resp := &rls.ListReleasesResponse{
		Count:    int64(len(c.rels)),
		Releases: c.rels,
	}
	return resp, c.err
}

func (c *fakeReleaseClient) InstallRelease(chStr string, opts ...helm.InstallOption) (*rls.InstallReleaseResponse, error) {
	return nil, nil
}

func (c *fakeReleaseClient) DeleteRelease(rlsName string, opts ...helm.DeleteOption) (*rls.UninstallReleaseResponse, error) {
	return nil, nil
}

func (c *fakeReleaseClient) ReleaseStatus(rlsName string, opts ...helm.StatusOption) (*rls.GetReleaseStatusResponse, error) {
	return nil, nil
}

func (c *fakeReleaseClient) UpdateRelease(rlsName string, opts ...helm.UpdateOption) (*rls.UpdateReleaseResponse, error) {
	return nil, nil
}

func (c *fakeReleaseClient) ReleaseContent(rlsName string, opts ...helm.ContentOption) (resp *rls.GetReleaseContentResponse, err error) {
	if len(c.rels) > 0 {
		resp = &rls.GetReleaseContentResponse{
			Release: c.rels[0],
		}
	}
	return resp, c.err
}
