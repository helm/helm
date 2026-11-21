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

package statusreaders

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newStorageClass(name string, mode storagev1.VolumeBindingMode, isDefault bool, created time.Time) storagev1.StorageClass {
	sc := storagev1.StorageClass{
		Name:              name,
		CreationTimestamp: metav1.NewTime(created),
		VolumeBindingMode: &mode,
	}
	if isDefault {
		sc.Annotations = map[string]string{defaultStorageClassAnnotation: "true"}
	}
	return sc
}

func TestWaitsForFirstConsumer(t *testing.T) {
	t.Parallel()
	now := time.Now()
	wffc := newStorageClass("wffc", storagev1.VolumeBindingWaitForFirstConsumer, false, now)
	immediate := newStorageClass("immediate", storagev1.VolumeBindingImmediate, false, now)
	noMode := storagev1.StorageClass{Name: "no-mode"}
	betaDefault := newStorageClass("beta-default", storagev1.VolumeBindingWaitForFirstConsumer, false, now)
	betaDefault.Annotations = map[string]string{betaDefaultStorageClassAnnotation: "true"}
	errLookup := errors.New("forbidden")

	tests := []struct {
		name             string
		storageClassName *string
		classes          []storagev1.StorageClass
		getErr, listErr  error
		want             bool
		wantErr          error
	}{
		{name: "named class waits for first consumer", storageClassName: new("wffc"), classes: []storagev1.StorageClass{wffc}, want: true},
		{name: "named class binds immediately", storageClassName: new("immediate"), classes: []storagev1.StorageClass{immediate}, want: false},
		{name: "named class without binding mode", storageClassName: new("no-mode"), classes: []storagev1.StorageClass{noMode}, want: false},
		{name: "empty name means no class", storageClassName: new(""), classes: []storagev1.StorageClass{wffc}, getErr: errLookup, listErr: errLookup, want: false},
		{name: "unset name uses the default class", classes: []storagev1.StorageClass{immediate, newStorageClass("default", storagev1.VolumeBindingWaitForFirstConsumer, true, now)}, want: true},
		{name: "unset name uses the beta default annotation", classes: []storagev1.StorageClass{betaDefault}, want: true},
		{name: "unset name without a default class", classes: []storagev1.StorageClass{wffc}, want: false},
		{
			name: "newest default class wins",
			classes: []storagev1.StorageClass{
				newStorageClass("old-default", storagev1.VolumeBindingImmediate, true, now.Add(-time.Hour)),
				newStorageClass("new-default", storagev1.VolumeBindingWaitForFirstConsumer, true, now),
			},
			want: true,
		},
		{name: "named class lookup error", storageClassName: new("wffc"), getErr: errLookup, wantErr: errLookup},
		{name: "default class lookup error", listErr: errLookup, wantErr: errLookup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			getClass := func(name string) (*storagev1.StorageClass, error) {
				if tt.getErr != nil {
					return nil, tt.getErr
				}
				for i := range tt.classes {
					if tt.classes[i].Name == name {
						return &tt.classes[i], nil
					}
				}
				return nil, errors.New("not found")
			}
			listClasses := func() ([]storagev1.StorageClass, error) {
				return tt.classes, tt.listErr
			}
			got, err := WaitsForFirstConsumer(tt.storageClassName, getClass, listClasses)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
