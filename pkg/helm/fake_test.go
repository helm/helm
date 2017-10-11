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

	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

func TestFakeClient_ReleaseStatus(t *testing.T) {
	releasePresent := ReleaseMock(&MockReleaseOptions{Name: "release-present"})
	releaseNotPresent := ReleaseMock(&MockReleaseOptions{Name: "release-not-present"})

	type fields struct {
		Rels []*release.Release
	}
	type args struct {
		rlsName string
		opts    []StatusOption
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *rls.GetReleaseStatusResponse
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
				opts:    nil,
			},
			want: &rls.GetReleaseStatusResponse{
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
				opts:    nil,
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
				opts:    nil,
			},
			want: &rls.GetReleaseStatusResponse{
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
			got, err := c.ReleaseStatus(tt.args.rlsName, tt.args.opts...)
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
		want      *rls.InstallReleaseResponse
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
			want: &rls.InstallReleaseResponse{
				Release: ReleaseMock(&MockReleaseOptions{Name: "new-release"}),
			},
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

func TestFakeClient_DeleteRelease(t *testing.T) {
	type fields struct {
		Rels []*release.Release
	}
	type args struct {
		rlsName string
		opts    []DeleteOption
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *rls.UninstallReleaseResponse
		relsAfter []*release.Release
		wantErr   bool
	}{
		{
			name: "Delete a release that exists.",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
					ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
				},
			},
			args: args{
				rlsName: "trepid-tapir",
				opts:    []DeleteOption{},
			},
			relsAfter: []*release.Release{
				ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
			},
			want: &rls.UninstallReleaseResponse{
				Release: ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
			},
			wantErr: false,
		},
		{
			name: "Delete a release that does not exist.",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
					ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
				},
			},
			args: args{
				rlsName: "release-that-does-not-exists",
				opts:    []DeleteOption{},
			},
			relsAfter: []*release.Release{
				ReleaseMock(&MockReleaseOptions{Name: "angry-dolphin"}),
				ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Delete when only 1 item exists.",
			fields: fields{
				Rels: []*release.Release{
					ReleaseMock(&MockReleaseOptions{Name: "trepid-tapir"}),
				},
			},
			args: args{
				rlsName: "trepid-tapir",
				opts:    []DeleteOption{},
			},
			relsAfter: []*release.Release{},
			want: &rls.UninstallReleaseResponse{
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
			got, err := c.DeleteRelease(tt.args.rlsName, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("FakeClient.DeleteRelease() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FakeClient.DeleteRelease() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(c.Rels, tt.relsAfter) {
				t.Errorf("FakeClient.InstallReleaseFromChart() rels = %v, expected %v", got, tt.relsAfter)
			}
		})
	}
}
