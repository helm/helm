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

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/statusreaders"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
)

type customPVCStatusReader struct {
	genericStatusReader engine.StatusReader
}

// NewCustomPVCStatusReader returns a status reader for PersistentVolumeClaims
// that reports a Pending claim as Current when its StorageClass only binds
// volumes once a Pod uses the claim (volumeBindingMode WaitForFirstConsumer).
// Otherwise it reports the generic kstatus status, which requires the claim to
// be Bound. If the StorageClass cannot be read, the claim keeps waiting for
// Bound.
func NewCustomPVCStatusReader(mapper meta.RESTMapper) engine.StatusReader {
	return &customPVCStatusReader{
		genericStatusReader: statusreaders.NewGenericStatusReader(mapper, status.Compute),
	}
}

func (p *customPVCStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim").GroupKind()
}

func (p *customPVCStatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader, resource object.ObjMetadata) (*event.ResourceStatus, error) {
	rs, err := p.genericStatusReader.ReadStatus(ctx, reader, resource)
	if err != nil {
		return rs, err
	}
	return readyIfWaitingForFirstConsumer(ctx, reader, rs), nil
}

func (p *customPVCStatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, resource *unstructured.Unstructured) (*event.ResourceStatus, error) {
	rs, err := p.genericStatusReader.ReadStatusForObject(ctx, reader, resource)
	if err != nil {
		return rs, err
	}
	return readyIfWaitingForFirstConsumer(ctx, reader, rs), nil
}

func readyIfWaitingForFirstConsumer(ctx context.Context, reader engine.ClusterReader, rs *event.ResourceStatus) *event.ResourceStatus {
	if rs == nil || rs.Status != status.InProgressStatus || rs.Resource == nil {
		return rs
	}
	obj := rs.Resource.UnstructuredContent()
	if status.GetStringField(obj, ".status.phase", "") != string(corev1.ClaimPending) {
		return rs
	}
	var storageClassName *string
	if name, found, err := unstructured.NestedString(obj, "spec", "storageClassName"); err == nil && found {
		storageClassName = &name
	}
	waits, err := WaitsForFirstConsumer(storageClassName,
		func(name string) (*storagev1.StorageClass, error) {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(storagev1.SchemeGroupVersion.WithKind("StorageClass"))
			if err := reader.Get(ctx, client.ObjectKey{Name: name}, u); err != nil {
				return nil, err
			}
			return toStorageClass(u)
		},
		func() ([]storagev1.StorageClass, error) {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(storagev1.SchemeGroupVersion.WithKind("StorageClassList"))
			if err := reader.ListClusterScoped(ctx, list, labels.Everything()); err != nil {
				return nil, err
			}
			classes := make([]storagev1.StorageClass, 0, len(list.Items))
			for i := range list.Items {
				sc, err := toStorageClass(&list.Items[i])
				if err != nil {
					return nil, err
				}
				classes = append(classes, *sc)
			}
			return classes, nil
		})
	if err != nil || !waits {
		return rs
	}
	rs.Status = status.CurrentStatus
	rs.Message = "PVC is waiting for its first consumer to be bound"
	return rs
}

func toStorageClass(u *unstructured.Unstructured) (*storagev1.StorageClass, error) {
	sc := &storagev1.StorageClass{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), sc); err != nil {
		return nil, err
	}
	return sc, nil
}
