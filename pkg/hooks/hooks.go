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

package hooks

import (
	"k8s.io/helm/pkg/proto/hapi/release"
)

const (
	// HookAnno is the label name for a hook
	HookAnno = "helm.sh/hook"
	// HookWeightAnno is the label name for a hook weight
	HookWeightAnno = "helm.sh/hook-weight"
	// HookDeleteAnno is the label name for the delete policy for a hook
	HookDeleteAnno = "helm.sh/hook-delete-policy"
)

// Types of hooks
const (
	PreInstall         = "pre-install"
	PostInstall        = "post-install"
	PreDelete          = "pre-delete"
	PostDelete         = "post-delete"
	PreUpgrade         = "pre-upgrade"
	PostUpgrade        = "post-upgrade"
	PreRollback        = "pre-rollback"
	PostRollback       = "post-rollback"
	ReleaseTestSuccess = "test-success"
	ReleaseTestFailure = "test-failure"
	CRDInstall         = "crd-install"
)

// Type of policy for deleting the hook
const (
	HookSucceeded      = "hook-succeeded"
	HookFailed         = "hook-failed"
	BeforeHookCreation = "before-hook-creation"
)

// Events represents a mapping between the key in the annotation for hooks and
// the protobuf-defined IDs.
var Events = map[string]release.Hook_Event{
	PreInstall:         release.Hook_PRE_INSTALL,
	PostInstall:        release.Hook_POST_INSTALL,
	PreDelete:          release.Hook_PRE_DELETE,
	PostDelete:         release.Hook_POST_DELETE,
	PreUpgrade:         release.Hook_PRE_UPGRADE,
	PostUpgrade:        release.Hook_POST_UPGRADE,
	PreRollback:        release.Hook_PRE_ROLLBACK,
	PostRollback:       release.Hook_POST_ROLLBACK,
	ReleaseTestSuccess: release.Hook_RELEASE_TEST_SUCCESS,
	ReleaseTestFailure: release.Hook_RELEASE_TEST_FAILURE,
	CRDInstall:         release.Hook_CRD_INSTALL,
}

// DeletePolices represents a mapping between the key in the annotation for
// label deleting policy and the protobuf-defined IDs
var DeletePolices = map[string]release.Hook_DeletePolicy{
	HookSucceeded:      release.Hook_SUCCEEDED,
	HookFailed:         release.Hook_FAILED,
	BeforeHookCreation: release.Hook_BEFORE_HOOK_CREATION,
}

// FilterTestHooks filters the list of hooks are returns only testing hooks.
func FilterTestHooks(hooks []*release.Hook) []*release.Hook {
	testHooks := []*release.Hook{}

	for _, h := range hooks {
		for _, e := range h.Events {
			if e == release.Hook_RELEASE_TEST_SUCCESS || e == release.Hook_RELEASE_TEST_FAILURE {
				testHooks = append(testHooks, h)
				continue
			}
		}
	}

	return testHooks
}
