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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestNewBump(t *testing.T) {
	cfg := actionConfigFixture(t)
	client := NewBump(cfg)

	assert.NotNil(t, client)
	require.Equal(t, cfg, client.cfg)
}

func TestBump_Run(t *testing.T) {
	type variant struct {
		name        string
		bumpType    string
		wantVersion string
	}
	tests := []struct {
		originVersion string
		variant       []variant
		wantErr       bool
	}{
		{
			originVersion: "1.2.3",
			variant: []variant{
				{name: "Major bump with valid version", bumpType: "major", wantVersion: "2.0.0"},
				{name: "Minor bump with valid version", bumpType: "minor", wantVersion: "1.3.0"},
				{name: "Patch bump with valid version", bumpType: "patch", wantVersion: "1.2.4"},
				{name: "Dev bump with valid version", bumpType: "dev", wantVersion: "1.2.3-dev.1"},
				{name: "RC bump with valid version", bumpType: "rc", wantVersion: "1.2.3-rc.1"},
				{name: "Alpha bump with valid version", bumpType: "alpha", wantVersion: "1.2.3-alpha.1"},
				{name: "Post bump with valid version", bumpType: "post", wantVersion: "1.2.3-post.1"},
				{name: "Explicit version", bumpType: "2.0.1", wantVersion: "2.0.1"},
				{name: "Explicit version", bumpType: "1.8.9-post.1", wantVersion: "1.8.9-post.1"},
			},
			wantErr: false,
		},
		{
			originVersion: "1.2",
			variant: []variant{
				{name: "Major bump with invalid version", bumpType: "major", wantVersion: ""},
				{name: "Minor bump with invalid version", bumpType: "minor", wantVersion: ""},
				{name: "Patch bump with invalid version", bumpType: "patch", wantVersion: ""},
				{name: "Dev bump with invalid version", bumpType: "dev", wantVersion: ""},
				{name: "Beta bump with invalid version", bumpType: "beta", wantVersion: ""},
				{name: "RC bump with invalid version", bumpType: "rc", wantVersion: ""},
				{name: "Post bump with invalid version", bumpType: "post", wantVersion: ""},
			},
			wantErr: true,
		},
		{
			originVersion: "1.2.a",
			variant: []variant{
				{name: "Major bump with invalid version", bumpType: "major", wantVersion: ""},
				{name: "Minor bump with invalid version", bumpType: "minor", wantVersion: ""},
				{name: "Patch bump with invalid version", bumpType: "patch", wantVersion: ""},
				{name: "Dev bump with invalid version", bumpType: "dev", wantVersion: ""},
				{name: "Beta bump with invalid version", bumpType: "beta", wantVersion: ""},
				{name: "RC bump with invalid version", bumpType: "rc", wantVersion: ""},
				{name: "Post bump with invalid version", bumpType: "post", wantVersion: ""},
			},
			wantErr: true,
		},
		{
			originVersion: "1.a.3",
			variant: []variant{
				{name: "Major bump with invalid version", bumpType: "major", wantVersion: ""},
				{name: "Minor bump with invalid version", bumpType: "minor", wantVersion: ""},
				{name: "Patch bump with invalid version", bumpType: "patch", wantVersion: ""},
				{name: "Dev bump with invalid version", bumpType: "dev", wantVersion: ""},
				{name: "Beta bump with invalid version", bumpType: "beta", wantVersion: ""},
				{name: "RC bump with invalid version", bumpType: "rc", wantVersion: ""},
				{name: "Post bump with invalid version", bumpType: "post", wantVersion: ""},
				{name: "Explicit version", bumpType: "2.0.1", wantVersion: ""},
				{name: "Explicit version", bumpType: "1.8.9-post.1", wantVersion: ""},
			},
			wantErr: true,
		},
		{
			originVersion: "1.2.3-alpha",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Alpha bump with pre-release version", bumpType: "alpha", wantVersion: "1.2.3-alpha.1"},
			},
			wantErr: false,
		},
		{
			originVersion: "1.2.3-alpha.1",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Alpha bump with pre-release version", bumpType: "alpha", wantVersion: "1.2.3-alpha.2"},
				{name: "Explicit version", bumpType: "2.0.1", wantVersion: "2.0.1"},
				{name: "Explicit version", bumpType: "1.8.9-post.1", wantVersion: "1.8.9-post.1"},
			},
			wantErr: false,
		},
		{

			originVersion: "1.2.3-beta",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Beta bump with pre-release version", bumpType: "beta", wantVersion: "1.2.3-beta.1"},
			},
			wantErr: false,
		},
		{
			originVersion: "1.2.3-beta.1",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Beta bump with pre-release version", bumpType: "beta", wantVersion: "1.2.3-beta.2"},
				{name: "Explicit version", bumpType: "2.0.1", wantVersion: "2.0.1"},
			},
			wantErr: false,
		},
		{
			originVersion: "1.2.3-rc",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "RC bump with pre-release version", bumpType: "rc", wantVersion: "1.2.3-rc.1"},
			},
			wantErr: false,
		},
		{
			originVersion: "1.2.3-rc.1",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Rc bump with pre-release version", bumpType: "rc", wantVersion: "1.2.3-rc.2"},
				{name: "Explicit version", bumpType: "1.8.9-post.1", wantVersion: "1.8.9-post.1"},
			},
			wantErr: false,
		},
		{
			originVersion: "1.2.3-post",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Post bump with pre-release version", bumpType: "post", wantVersion: "1.2.3-post.1"},
			},
			wantErr: false,
		},
		{

			originVersion: "1.2.3-post.1",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "RC bump with pre-release version", bumpType: "post", wantVersion: "1.2.3-post.2"},
				{name: "Explicit version", bumpType: "2.0.1", wantVersion: "2.0.1"},
			},
			wantErr: false,
		},
		{
			originVersion: "1.2.3-dev",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Dev bump with pre-release version", bumpType: "dev", wantVersion: "1.2.3-dev.1"},
				{name: "Explicit version", bumpType: "2.0.1", wantVersion: "2.0.1"},
			},
			wantErr: false,
		},
		{

			originVersion: "1.2.3-dev.1",
			variant: []variant{
				{name: "Stable bump with pre-release version", bumpType: "stable", wantVersion: "1.2.3"},
				{name: "Dev bump with pre-release version", bumpType: "dev", wantVersion: "1.2.3-dev.2"},
				{name: "Explicit version", bumpType: "2.0.1", wantVersion: "2.0.1"},
				{name: "Explicit version", bumpType: "1.8.9-post.1", wantVersion: "1.8.9-post.1"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		for _, v := range tt.variant {
			t.Run(v.name, func(t *testing.T) {
				cfg := actionConfigFixture(t)
				client := NewBump(cfg)
				client.chart = &chart.Chart{
					Metadata: &chart.Metadata{Name: "test", Version: tt.originVersion},
				}

				result, err := client.Run(v.bumpType, "")
				if tt.wantErr {
					require.Error(t, err)
					require.Empty(t, result)
				} else {
					require.NoError(t, err)
					require.Equal(t, v.wantVersion, result)
				}
			})
		}
	}
}
