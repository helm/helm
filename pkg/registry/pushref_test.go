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

package registry // import "helm.sh/helm/v4/pkg/registry"

import (
	"reflect"
	"testing"
)

func TestBuildPushRef(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		chart    string
		version  string
		want     string
		wantErr  bool
	}{
		{
			name:     "simple case",
			registry: "oci://my-registry.io/my-repo:1.0.0",
			chart:    "my-repo",
			version:  "1.0.0",
			want:     "my-registry.io/my-repo:1.0.0",
			wantErr:  false,
		},
		{
			name:     "append chart name to repo",
			registry: "oci://my-registry.io/ns",
			chart:    "my-repo",
			version:  "1.0.0",
			want:     "my-registry.io/ns/my-repo:1.0.0",
			wantErr:  false,
		},
		{
			name:     "digest not allowed",
			registry: "oci://my-registry.io/my-repo@sha256:abcdef1234567890",
			chart:    "my-repo",
			version:  "1.0.0",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "invalid registry",
			registry: "invalid-registry",
			chart:    "my-repo",
			version:  "1.0.0",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "tag mismatch",
			registry: "oci://my-registry.io/my-repo:2.0.0",
			chart:    "my-repo",
			version:  "1.0.0",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "plus to underscore normalization",
			registry: "oci://my-registry.io/my-repo:1.0.0_abc",
			chart:    "my-repo",
			version:  "1.0.0+abc",
			want:     "my-registry.io/my-repo:1.0.0+abc",
			wantErr:  false,
		},
		{
			name:     "repo already includes chart name",
			registry: "oci://my-registry.io/namespace/my-repo:1.0.0",
			chart:    "my-repo",
			version:  "1.0.0",
			want:     "my-registry.io/namespace/my-repo:1.0.0",
		},
	}

	for _, tt := range tests {

		if tt.wantErr {
			t.Run(tt.name, func(t *testing.T) {
				_, err := BuildPushRef(tt.registry, tt.chart, tt.version)
				if err == nil {
					t.Errorf("BuildPushRef() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			})
		} else {
			t.Run(tt.name, func(t *testing.T) {
				got, err := BuildPushRef(tt.registry, tt.chart, tt.version)
				if err != nil {
					t.Errorf("BuildPushRef() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("BuildPushRef() got = %v, want %v", got, tt.want)
				}
			})
		}
	}
}
