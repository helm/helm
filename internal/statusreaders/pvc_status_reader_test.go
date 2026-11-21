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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/cli-utils/pkg/testutil"
)

// fakeStorageClassReader serves StorageClasses, and optionally the claim
// itself, to the status reader.
type fakeStorageClassReader struct {
	classes []storagev1.StorageClass
	pvc     *unstructured.Unstructured
	err     error
}

func (f *fakeStorageClassReader) Get(_ context.Context, key client.ObjectKey, obj *unstructured.Unstructured) error {
	if f.pvc != nil && obj.GetKind() == "PersistentVolumeClaim" {
		obj.SetUnstructuredContent(f.pvc.DeepCopy().UnstructuredContent())
		return nil
	}
	if f.err != nil {
		return f.err
	}
	for i := range f.classes {
		if f.classes[i].Name == key.Name {
			u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&f.classes[i])
			if err != nil {
				return err
			}
			obj.SetUnstructuredContent(u)
			return nil
		}
	}
	return errors.New("not found")
}

func (f *fakeStorageClassReader) ListNamespaceScoped(context.Context, *unstructured.UnstructuredList, string, labels.Selector) error {
	return errors.New("not implemented")
}

func (f *fakeStorageClassReader) ListClusterScoped(_ context.Context, list *unstructured.UnstructuredList, _ labels.Selector) error {
	if f.err != nil {
		return f.err
	}
	for i := range f.classes {
		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&f.classes[i])
		if err != nil {
			return err
		}
		list.Items = append(list.Items, unstructured.Unstructured{Object: u})
	}
	return nil
}

func (f *fakeStorageClassReader) Sync(context.Context) error {
	return nil
}

func newPVC(t *testing.T, phase corev1.PersistentVolumeClaimPhase, storageClassName *string) *unstructured.Unstructured {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{
		Spec:   corev1.PersistentVolumeClaimSpec{StorageClassName: storageClassName},
		Status: corev1.PersistentVolumeClaimStatus{Phase: phase},
	}
	pvc.SetName("data")
	pvc.SetNamespace("default")
	pvc.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim"))
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pvc)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: u}
}

func TestPVCStatusReader(t *testing.T) {
	t.Parallel()
	now := time.Now()
	classes := []storagev1.StorageClass{
		newStorageClass("wffc", storagev1.VolumeBindingWaitForFirstConsumer, true, now),
		newStorageClass("immediate", storagev1.VolumeBindingImmediate, false, now),
	}

	tests := []struct {
		name   string
		pvc    *unstructured.Unstructured
		reader *fakeStorageClassReader
		want   status.Status
	}{
		{name: "bound claim is current", pvc: newPVC(t, corev1.ClaimBound, new("immediate")), reader: &fakeStorageClassReader{classes: classes}, want: status.CurrentStatus},
		{name: "pending claim with WaitForFirstConsumer class is current", pvc: newPVC(t, corev1.ClaimPending, new("wffc")), reader: &fakeStorageClassReader{classes: classes}, want: status.CurrentStatus},
		{name: "pending claim with the default WaitForFirstConsumer class is current", pvc: newPVC(t, corev1.ClaimPending, nil), reader: &fakeStorageClassReader{classes: classes}, want: status.CurrentStatus},
		{name: "pending claim with Immediate class is in progress", pvc: newPVC(t, corev1.ClaimPending, new("immediate")), reader: &fakeStorageClassReader{classes: classes}, want: status.InProgressStatus},
		{name: "pending claim without a class is in progress", pvc: newPVC(t, corev1.ClaimPending, new("")), reader: &fakeStorageClassReader{classes: classes}, want: status.InProgressStatus},
		{name: "pending claim whose class cannot be read is in progress", pvc: newPVC(t, corev1.ClaimPending, new("wffc")), reader: &fakeStorageClassReader{err: errors.New("forbidden")}, want: status.InProgressStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sr := NewCustomPVCStatusReader(testutil.NewFakeRESTMapper(corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")))
			rs, err := sr.ReadStatusForObject(context.Background(), tt.reader, tt.pvc)
			require.NoError(t, err)
			assert.Equal(t, tt.want, rs.Status)
		})
	}
}

func TestPVCStatusReaderReadStatus(t *testing.T) {
	t.Parallel()
	pvc := newPVC(t, corev1.ClaimPending, new("wffc"))
	reader := &fakeStorageClassReader{
		classes: []storagev1.StorageClass{newStorageClass("wffc", storagev1.VolumeBindingWaitForFirstConsumer, false, time.Now())},
		pvc:     pvc,
	}
	sr := NewCustomPVCStatusReader(testutil.NewFakeRESTMapper(corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")))
	rs, err := sr.ReadStatus(context.Background(), reader, object.UnstructuredToObjMetadata(pvc))
	require.NoError(t, err)
	assert.Equal(t, status.CurrentStatus, rs.Status)
}

func TestPVCStatusReaderSupports(t *testing.T) {
	t.Parallel()
	sr := NewCustomPVCStatusReader(testutil.NewFakeRESTMapper())
	assert.True(t, sr.Supports(corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim").GroupKind()))
	assert.False(t, sr.Supports(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind()))
}
