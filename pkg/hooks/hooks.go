/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"errors"

	"k8s.io/helm/pkg/proto/hapi/release"
)

// HookAnno is the label name for a hook
const HookAnno = "helm.sh/hook"

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
)

// FilterTestHooks filters the list of hooks are returns only testing hooks.
func FilterTestHooks(hooks []*release.Hook) ([]*release.Hook, error) {
	testHooks := []*release.Hook{}
	notFoundErr := errors.New("no tests found")

	if len(hooks) == 0 {
		return nil, notFoundErr
	}

	for _, h := range hooks {
		for _, e := range h.Events {
			if e == release.Hook_RELEASE_TEST_SUCCESS || e == release.Hook_RELEASE_TEST_FAILURE {
				testHooks = append(testHooks, h)
				continue
			}
		}
	}

	if len(testHooks) == 0 {
		return nil, notFoundErr
	}

	return testHooks, nil
}
