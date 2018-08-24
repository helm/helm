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

package helm

import (
	"reflect"
	"testing"

	"k8s.io/helm/pkg/chart"
	"k8s.io/helm/pkg/hapi"
	"k8s.io/helm/pkg/hapi/release"
)

func TestFakeClient_ReleaseStatus(t *testing.T) {
	releasePresent := ReleaseMock(&MockReleaseOptions{Name: "release-present"})
	releaseNotPresent := ReleaseMock(&MockReleaseOptions{Name: "release-not-present"})

	type fields struct {
		Rels []*release.Release
	}
	type args struct {
		rlsName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *hapi.GetReleaseStatusResponse
		wantErr bool
	}{
		{
			name: "Get a single release that exists",
			fields: fields{
				Rels: []*release.Release{
					releasePresent,
				},
			},
			args: args{
				rlsName: releasePresent.Name,
			},
			want: &hapi.GetReleaseStatusResponse{
				Name:      releasePresent.Name,
				Info:      releasePresent.Info,
				Namespace: releasePresent.Namespace,
			},

			wantErr: false,
		},
		{
			name: "Get a release that does not exist",
			fields: fields{
				Rels: []*release.Release{
					releasePresent,
				},
			},
			args: args{
				rlsName: releaseNotPresent.Name,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Get a single release that exists from list",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin", Namespace: "default"}),
					ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir", Namespace: "default"}),
					releasePresent,
				},
			},
			args: args{
				rlsName: releasePresent.Name,
			},
			want: &hapi.GetReleaseStatusResponse{
				Name:      releasePresent.Name,
				Info:      releasePresent.Info,
				Namespace: releasePresent.Namespace,
			},

			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FakeClient{
				Rels: tt.fields.Rels,
			}
			got, err := c.ReleaseStatus(tt.args.rlsName, 0)
			if (err != nil) != tt.wantErr {
				t.Errorf("FakeClient.ReleaseStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FakeClient.ReleaseStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFakeClient_InstallReleaseFromChart(t *testing.T) {
	installChart := &chart.Chart{}
	type fields struct {
		Rels []*release.Release
	}
	type args struct {
		ns   string
		opts []InstallOption
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *release.Release
		relsAfter []*release.Release
		wantErr   bool
	}{
		{
			name: "Add release to an empty list.",
			fields: fields{
				Rels: []*release.Release{},
			},
			args: args{
				ns:   "default",
				opts: []InstallOption{ReleaseName("new-release")},
			},
			want: ReleaseMock(&MockReleaseOptions{Name: "new-release"}),
			relsAfter: []*release.Release{
				ReleaseMock(&MockReleaseOptions{Name: "new-release"}),
			},
			wantErr: false,
		},
		{
			name: "Try to add a release where the name already exists.",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "new-release"}),
				},
			},
			args: args{
				ns:   "default",
				opts: []InstallOption{ReleaseName("new-release")},
			},
			relsAfter: []*release.Release{
				ReleaseMock(&MockReleaseOptions{Name: "new-release"}),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FakeClient{
				Rels: tt.fields.Rels,
			}
			got, err := c.InstallReleaseFromChart(installChart, tt.args.ns, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("FakeClient.InstallReleaseFromChart() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FakeClient.InstallReleaseFromChart() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(c.Rels, tt.relsAfter) {
				t.Errorf("FakeClient.InstallReleaseFromChart() rels = %v, expected %v", got, tt.relsAfter)
			}
		})
	}
}

func TestFakeClient_UninstallRelease(t *testing.T) {
	type fields struct {
		Rels []*release.Release
	}
	type args struct {
		rlsName string
		opts    []UninstallOption
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *hapi.UninstallReleaseResponse
		relsAfter []*release.Release
		wantErr   bool
	}{
		{
			name: "Uninstall a release that exists.",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
					ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
				},
			},
			args: args{
				rlsName: "trepid-tapir",
				opts:    []UninstallOption{},
			},
			relsAfter: []*release.Release{
				ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
			},
			want: &hapi.UninstallReleaseResponse{
				Release: ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
			},
			wantErr: false,
		},
		{
			name: "Uninstall a release that does not exist.",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
					ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
				},
			},
			args: args{
				rlsName: "release-that-does-not-exists",
				opts:    []UninstallOption{},
			},
			relsAfter: []*release.Release{
				ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
				ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Uninstall when only 1 item exists.",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
				},
			},
			args: args{
				rlsName: "trepid-tapir",
				opts:    []UninstallOption{},
			},
			relsAfter: []*release.Release{},
			want: &hapi.UninstallReleaseResponse{
				Release: ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FakeClient{
				Rels: tt.fields.Rels,
			}
			got, err := c.UninstallRelease(tt.args.rlsName, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("FakeClient.UninstallRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FakeClient.UninstallRelease() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(c.Rels, tt.relsAfter) {
				t.Errorf("FakeClient.InstallReleaseFromChart() rels = %v, expected %v", got, tt.relsAfter)
			}
		})
	}
}
