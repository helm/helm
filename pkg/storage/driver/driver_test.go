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

package driver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"helm.sh/helm/v4/pkg/release/common"
)

func TestUpdateMissingRelease(t *testing.T) {
	tests := []struct {
		name      string
		newDriver func(*testing.T) Driver
	}{
		{"memory", func(*testing.T) Driver { return NewMemory() }},
		{"configmaps", func(t *testing.T) Driver {
			t.Helper()
			return newTestFixtureCfgMaps(t)
		}},
		{"secrets", func(t *testing.T) Driver {
			t.Helper()
			return newTestFixtureSecrets(t)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.newDriver(t)
			rel := releaseStub("missing", 1, "default", common.StatusDeployed)
			key := testKey(rel.Name, rel.Version)
			require.ErrorIs(t, d.Update(key, rel), ErrReleaseNotFound)
			// An update must not create the missing release.
			_, err := d.Get(key)
			require.ErrorIs(t, err, ErrReleaseNotFound)
		})
	}
}

func TestUpdateKubernetesErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"forbidden", apierrors.NewForbidden(v1.Resource("tests"), "test", errors.New("access denied"))},
		{"conflict", apierrors.NewConflict(v1.Resource("tests"), "test", errors.New("stale resource version"))},
		{"connection", errors.New("connection failed")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drivers := []Driver{
				NewConfigMaps(&MockConfigMapsInterface{updateError: tt.err}),
				NewSecrets(&MockSecretsInterface{updateError: tt.err}),
			}
			for _, d := range drivers {
				t.Run(d.Name(), func(t *testing.T) {
					rel := releaseStub("test", 1, "default", common.StatusDeployed)
					err := d.Update(testKey(rel.Name, rel.Version), rel)
					require.ErrorIs(t, err, tt.err)
					require.NotErrorIs(t, err, ErrReleaseNotFound)
				})
			}
		})
	}
}
