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

package release

import "time"

// HookEvent specifies the hook event
type HookEvent string

// Hook event types
const (
	HookPreInstall         HookEvent = "pre-install"
	HookPostInstall        HookEvent = "post-install"
	HookPreDelete          HookEvent = "pre-delete"
	HookPostDelete         HookEvent = "post-delete"
	HookPreUpgrade         HookEvent = "pre-upgrade"
	HookPostUpgrade        HookEvent = "post-upgrade"
	HookPreRollback        HookEvent = "pre-rollback"
	HookPostRollback       HookEvent = "post-rollback"
	HookReleaseTestSuccess HookEvent = "release-test-success"
	HookReleaseTestFailure HookEvent = "release-test-failure"
)

func (x HookEvent) String() string { return string(x) }

// HookDeletePolicy specifies the hook delete policy
type HookDeletePolicy string

// Hook delete policy types
const (
	HookSucceeded          HookDeletePolicy = "succeeded"
	HookFailed             HookDeletePolicy = "failed"
	HookBeforeHookCreation HookDeletePolicy = "before-hook-creation"
)

func (x HookDeletePolicy) String() string { return string(x) }

// Hook defines a hook object.
type Hook struct {
	Name string `json:"name,omitempty"`
	// Kind is the Kubernetes kind.
	Kind string `json:"kind,omitempty"`
	// Path is the chart-relative path to the template.
	Path string `json:"path,omitempty"`
	// Manifest is the manifest contents.
	Manifest string `json:"manifest,omitempty"`
	// Events are the events that this hook fires on.
	Events []HookEvent `json:"events,omitempty"`
	// LastRun indicates the date/time this was last run.
	LastRun time.Time `json:"last_run,omitempty"`
	// Weight indicates the sort order for execution among similar Hook type
	Weight int `json:"weight,omitempty"`
	// DeletePolicies are the policies that indicate when to delete the hook
	DeletePolicies []HookDeletePolicy `json:"delete_policies,omitempty"`
}
