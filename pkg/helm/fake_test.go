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

package helm

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

const cmInputTemplate = `kind: ConfigMap
apiVersion: v1
metadata:
  name: example
data:
  Release:
{{.Release | toYaml | indent 4}}
`
const cmOutputTemplate = `
---
# Source: installChart/templates/cm.yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: example
data:
  Release:
    IsInstall: %t
    IsUpgrade: %t
    Name: new-release
    Namespace: default
    Revision: %d
    Service: Tiller
    Time:
      seconds: 242085845
    
`

var installChart *chart.Chart

func init() {
	installChart = &chart.Chart{
		Metadata: &chart.Metadata{Name: "installChart"},
		Templates: []*chart.Template{
			{Name: "templates/cm.yaml", Data: []byte(cmInputTemplate)},
		},
	}
}

func releaseWithChart(opts *MockReleaseOptions) *release.Release {
	if opts.Chart == nil {
		opts.Chart = installChart
	}
	return ReleaseMock(opts)
}

func withManifest(r *release.Release, isUpgrade bool) *release.Release {
	r.Manifest = fmt.Sprintf(cmOutputTemplate, !isUpgrade, isUpgrade, r.Version)
	return r
}

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
	type fields struct {
		Rels            []*release.Release
		RenderManifests bool
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
				Release: releaseWithChart(&MockReleaseOptions{Name: "new-release"}),
			},
			relsAfter: []*release.Release{
				releaseWithChart(&MockReleaseOptions{Name: "new-release"}),
			},
			wantErr: false,
		},
		{
			name: "Add release with description.",
			fields: fields{
				Rels: []*release.Release{},
			},
			args: args{
				ns:   "default",
				opts: []InstallOption{ReleaseName("new-release"), InstallDescription("foo-bar")},
			},
			want: &rls.InstallReleaseResponse{
				Release: releaseWithChart(&MockReleaseOptions{Name: "new-release", Description: "foo-bar"}),
			},
			relsAfter: []*release.Release{
				releaseWithChart(&MockReleaseOptions{Name: "new-release", Description: "foo-bar"}),
			},
			wantErr: false,
		},
		{
			name: "Try to add a release where the name already exists.",
			fields: fields{
				Rels: []*release.Release{
					releaseWithChart(&MockReleaseOptions{Name: "new-release"}),
				},
			},
			args: args{
				ns:   "default",
				opts: []InstallOption{ReleaseName("new-release")},
			},
			relsAfter: []*release.Release{
				releaseWithChart(&MockReleaseOptions{Name: "new-release"}),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Render the given chart.",
			fields: fields{
				Rels:            []*release.Release{},
				RenderManifests: true,
			},
			args: args{
				ns:   "default",
				opts: []InstallOption{ReleaseName("new-release")},
			},
			want: &rls.InstallReleaseResponse{
				Release: withManifest(releaseWithChart(&MockReleaseOptions{Name: "new-release"}), false),
			},
			relsAfter: []*release.Release{
				withManifest(releaseWithChart(&MockReleaseOptions{Name: "new-release"}), false),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FakeClient{
				Rels:            tt.fields.Rels,
				RenderManifests: tt.fields.RenderManifests,
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
				t.Errorf("FakeClient.InstallReleaseFromChart() rels = %v, expected %v", c.Rels, tt.relsAfter)
			}
		})
	}
}

func TestFakeClient_UpdateReleaseFromChart(t *testing.T) {
	type fields struct {
		Rels            []*release.Release
		RenderManifests bool
	}
	type args struct {
		release string
		opts    []UpdateOption
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		want      *rls.UpdateReleaseResponse
		relsAfter []*release.Release
		wantErr   bool
	}{
		{
			name: "Update release.",
			fields: fields{
				Rels: []*release.Release{
					releaseWithChart(&MockReleaseOptions{Name: "new-release"}),
				},
			},
			args: args{
				release: "new-release",
				opts:    []UpdateOption{},
			},
			want: &rls.UpdateReleaseResponse{
				Release: releaseWithChart(&MockReleaseOptions{Name: "new-release", Version: 2}),
			},
			relsAfter: []*release.Release{
				releaseWithChart(&MockReleaseOptions{Name: "new-release", Version: 2}),
			},
		},
		{
			name: "Update and render given chart.",
			fields: fields{
				Rels: []*release.Release{
					releaseWithChart(&MockReleaseOptions{Name: "new-release"}),
				},
				RenderManifests: true,
			},
			args: args{
				release: "new-release",
				opts:    []UpdateOption{},
			},
			want: &rls.UpdateReleaseResponse{
				Release: withManifest(releaseWithChart(&MockReleaseOptions{Name: "new-release", Version: 2}), true),
			},
			relsAfter: []*release.Release{
				withManifest(releaseWithChart(&MockReleaseOptions{Name: "new-release", Version: 2}), true),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FakeClient{
				Rels:            tt.fields.Rels,
				RenderManifests: tt.fields.RenderManifests,
			}
			got, err := c.UpdateReleaseFromChart(tt.args.release, installChart, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("FakeClient.UpdateReleaseFromChart() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FakeClient.UpdateReleaseFromChart() =\n%v\nwant\n%v", got, tt.want)
			}
			if !reflect.DeepEqual(c.Rels, tt.relsAfter) {
				t.Errorf("FakeClient.UpdateReleaseFromChart() rels =\n%v\nwant\n%v", c.Rels, tt.relsAfter)
			}
		})
	}
}
