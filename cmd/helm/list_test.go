package main

import (
	"bytes"
	"testing"

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
		Config: &chart.Config{Raw: `name = "value"`},
	}
}

type fakeReleaseLister struct {
	helm.Interface
	rels []*release.Release
	err  error
}

func (fl *fakeReleaseLister) ListReleases(opts ...helm.ReleaseListOption) (*rls.ListReleasesResponse, error) {
	resp := &rls.ListReleasesResponse{
		Count:    int64(len(fl.rels)),
		Releases: fl.rels,
	}
	return resp, fl.err
}

func TestListRun(t *testing.T) {
	tests := []struct {
		name     string
		lister   *lister
		expected string
		err      bool
	}{
		{
			name: "with a release",
			lister: &lister{
				client: &fakeReleaseLister{
					rels: []*release.Release{
						releaseMock("thomas-guide"),
					},
				},
			},
			expected: "thomas-guide",
		},
		{
			name: "list --long",
			lister: &lister{
				client: &fakeReleaseLister{
					rels: []*release.Release{
						releaseMock("atlas"),
					},
				},
				long: true,
			},
			expected: "NAME \tUPDATED                 \tSTATUS  \tCHART           \natlas\tFri Sep  2 15:04:05 1977\tDEPLOYED\tfoo-0.1.0-beta.1",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		tt.lister.out = &buf
		err := tt.lister.run()
		if (err != nil) != tt.err {
			t.Errorf("%q. expected error: %v, got %v", tt.name, tt.err, err)
		}
		actual := string(bytes.TrimSpace(buf.Bytes()))
		if actual != tt.expected {
			t.Errorf("%q. expected %q, got %q", tt.name, tt.expected, actual)
		}
		buf.Reset()
	}
}

func TestListCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flags    map[string]string
		client   helm.Interface
		expected string
		err      bool
	}{
		{
			name: "with a release",
			client: &fakeReleaseLister{
				rels: []*release.Release{
					releaseMock("thomas-guide"),
				},
			},
			expected: "thomas-guide",
		},
		{
			name:  "list --long",
			flags: map[string]string{"long": "1"},
			client: &fakeReleaseLister{
				rels: []*release.Release{
					releaseMock("atlas"),
				},
			},
			expected: "NAME \tUPDATED                 \tSTATUS  \tCHART           \natlas\tFri Sep  2 15:04:05 1977\tDEPLOYED\tfoo-0.1.0-beta.1",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		cmd := newListCmd(tt.client, &buf)
		for flag, value := range tt.flags {
			cmd.Flags().Set(flag, value)
		}
		err := cmd.RunE(cmd, tt.args)
		if (err != nil) != tt.err {
			t.Errorf("%q. expected error: %v, got %v", tt.name, tt.err, err)
		}
		actual := string(bytes.TrimSpace(buf.Bytes()))
		if actual != tt.expected {
			t.Errorf("%q. expected %q, got %q", tt.name, tt.expected, actual)
		}
		buf.Reset()
	}
}
