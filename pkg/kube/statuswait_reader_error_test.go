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
package kube

import (
	"context"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/cli-utils/pkg/testutil"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/kubectl/pkg/scheme"
)

// errStatusReader mimics cli-utils' errResourceToResourceStatus: the reader
// could not read something it needs (here a Forbidden list) and reports
// Unknown with the error attached.
type errStatusReader struct{ gk schema.GroupKind }

func (r *errStatusReader) Supports(gk schema.GroupKind) bool { return gk == r.gk }

func (r *errStatusReader) status(id object.ObjMetadata) *event.ResourceStatus {
	return &event.ResourceStatus{
		Identifier: id,
		Status:     status.UnknownStatus,
		Error: apierrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "replicasets"}, "",
			context.DeadlineExceeded),
	}
}

func (r *errStatusReader) ReadStatus(_ context.Context, _ engine.ClusterReader, id object.ObjMetadata) (*event.ResourceStatus, error) {
	return r.status(id), nil
}

func (r *errStatusReader) ReadStatusForObject(_ context.Context, _ engine.ClusterReader, u *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return r.status(object.UnstructuredToObjMetadata(u)), nil
}

// A resource whose status read fails must not be reported as ready.
func TestStatusWaitReaderErrorIsNotReady(t *testing.T) {
	c := newTestClient(t)
	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	fakeMapper := testutil.NewFakeRESTMapper(v1.SchemeGroupVersion.WithKind("Pod"))
	sw := statusWaiter{
		client:     fakeClient,
		restMapper: fakeMapper,
		readers:    []engine.StatusReader{&errStatusReader{gk: v1.SchemeGroupVersion.WithKind("Pod").GroupKind()}},
	}
	objs := getRuntimeObjFromManifests(t, []string{podNoStatusManifest})
	for _, obj := range objs {
		u := obj.(*unstructured.Unstructured)
		require.NoError(t, fakeClient.Tracker().Create(getGVR(t, fakeMapper, u), u, u.GetNamespace()))
	}
	err := sw.Wait(getResourceListFromRuntimeObjs(t, c, objs), 2*time.Second)
	require.Error(t, err, "Wait reported a resource as ready although its status could not be read")
	t.Logf("Wait error: %v", err)
}
