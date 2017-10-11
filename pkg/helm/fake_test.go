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

	"k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

func TestFakeClient_ReleaseStatus(t *testing.T) {
	releasePresent := &release.Release{Name: "release-present", Namespace: "default"}
	releaseNotPresent := &release.Release{Name: "release-not-present", Namespace: "default"}

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
					&release.Release{Name: "angry-dolphin", Namespace: "default"},
					&release.Release{Name: "trepid-tapir", Namespace: "default"},
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
