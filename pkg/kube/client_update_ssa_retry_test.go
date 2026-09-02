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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/client-go/rest/fake"
	"k8s.io/client-go/util/retry"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

// TestUpdateServerSideApplyRetriesOnResourceQuotaConflict is a regression
// test for the *update* (patch) path of server-side apply.
//
// PR #32088 (fixing issue #32087) added a retry.OnError wrapper around
// patchResourceServerSide in makeCreateApplyFunc, so that transient 409
// conflicts on a ResourceQuota's optimistic lock (see
// https://github.com/kubernetes/kubernetes/issues/67761) are retried when
// *creating* a resource via server-side apply.
//
// The exact same patchResourceServerSide call also happens in
// makeUpdateApplyFunc (used by every `helm upgrade` that patches an
// already-existing resource), but that call site was not given the same
// retry wrapper. This test proves that gap: a resourcequota conflict on the
// update path currently fails immediately instead of retrying.
func TestUpdateServerSideApplyRetriesOnResourceQuotaConflict(t *testing.T) {
	c := newTestClient(t)

	pods := newPodList("otter")

	cb := func(_ []RequestResponseAction, req *http.Request) (*http.Response, error) {
		p, m := req.URL.Path, req.Method
		switch {
		case p == "/namespaces/default/pods/otter" && m == http.MethodGet:
			return newResponse(http.StatusOK, &pods.Items[0])
		case p == "/namespaces/default/pods/otter" && m == http.MethodPatch:
			return newResponseJSON(http.StatusConflict, resourceQuotaConflict)
		}
		t.Fatalf("unexpected request: %s %s", m, p)
		return nil, nil
	}

	client := NewRequestResponseLogClient(t, cb)
	c.Factory.(*cmdtesting.TestFactory).UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: unstructuredSerializer,
		Client:               fake.CreateHTTPClient(client.Do),
	}

	originals, err := c.Build(objBody(&pods), false)
	require.NoError(t, err)
	targets, err := c.Build(objBody(&pods), false)
	require.NoError(t, err)

	_, err = c.Update(
		originals,
		targets,
		ClientUpdateOptionServerSideApply(true, false),
	)
	require.ErrorContains(t, err, "Operation cannot be fulfilled on resourcequotas")

	patchCount := 0
	for _, action := range client.Actions {
		if action.Request.URL.Path == "/namespaces/default/pods/otter" && action.Request.Method == http.MethodPatch {
			patchCount++
		}
	}

	assert.Equal(t, retry.DefaultRetry.Steps, patchCount,
		"expected helm to retry the server-side apply PATCH on a resourcequota conflict during update, the same way it already does during create")
}
