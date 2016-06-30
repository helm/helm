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
		Config: &chart.Config{Raw: `name: "value"`},
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
