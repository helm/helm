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

package v1

import (
	"encoding/json"
	"time"
)

// HookEvent specifies the hook event
type HookEvent string

// Hook event types
const (
	HookPreInstall   HookEvent = "pre-install"
	HookPostInstall  HookEvent = "post-install"
	HookPreDelete    HookEvent = "pre-delete"
	HookPostDelete   HookEvent = "post-delete"
	HookPreUpgrade   HookEvent = "pre-upgrade"
	HookPostUpgrade  HookEvent = "post-upgrade"
	HookPreRollback  HookEvent = "pre-rollback"
	HookPostRollback HookEvent = "post-rollback"
	HookTest         HookEvent = "test"
)

func (x HookEvent) String() string { return string(x) }

// HookDeletePolicy specifies the hook delete policy
type HookDeletePolicy string

// Hook delete policy types
const (
	HookSucceeded          HookDeletePolicy = "hook-succeeded"
	HookFailed             HookDeletePolicy = "hook-failed"
	HookBeforeHookCreation HookDeletePolicy = "before-hook-creation"
)

func (x HookDeletePolicy) String() string { return string(x) }

// HookOutputLogPolicy specifies the hook output log policy
type HookOutputLogPolicy string

// Hook output log policy types
const (
	HookOutputOnSucceeded HookOutputLogPolicy = "hook-succeeded"
	HookOutputOnFailed    HookOutputLogPolicy = "hook-failed"
)

func (x HookOutputLogPolicy) String() string { return string(x) }

// HookAnnotation is the label name for a hook
const HookAnnotation = "helm.sh/hook"

// HookWeightAnnotation is the label name for a hook weight
const HookWeightAnnotation = "helm.sh/hook-weight"

// HookDeleteAnnotation is the label name for the delete policy for a hook
const HookDeleteAnnotation = "helm.sh/hook-delete-policy"

// HookOutputLogAnnotation is the label name for the output log policy for a hook
const HookOutputLogAnnotation = "helm.sh/hook-output-log-policy"

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
	LastRun HookExecution `json:"last_run,omitempty"`
	// Weight indicates the sort order for execution among similar Hook type
	Weight int `json:"weight,omitempty"`
	// DeletePolicies are the policies that indicate when to delete the hook
	DeletePolicies []HookDeletePolicy `json:"delete_policies,omitempty"`
	// OutputLogPolicies defines whether we should copy hook logs back to main process
	OutputLogPolicies []HookOutputLogPolicy `json:"output_log_policies,omitempty"`
}

// A HookExecution records the result for the last execution of a hook for a given release.
type HookExecution struct {
	// StartedAt indicates the date/time this hook was started
	StartedAt time.Time `json:"started_at,omitzero"`
	// CompletedAt indicates the date/time this hook was completed.
	CompletedAt time.Time `json:"completed_at,omitzero"`
	// Phase indicates whether the hook completed successfully
	Phase HookPhase `json:"phase"`
}

// A HookPhase indicates the state of a hook execution
type HookPhase string

const (
	// HookPhaseUnknown indicates that a hook is in an unknown state
	HookPhaseUnknown HookPhase = "Unknown"
	// HookPhaseRunning indicates that a hook is currently executing
	HookPhaseRunning HookPhase = "Running"
	// HookPhaseSucceeded indicates that hook execution succeeded
	HookPhaseSucceeded HookPhase = "Succeeded"
	// HookPhaseFailed indicates that hook execution failed
	HookPhaseFailed HookPhase = "Failed"
)

// String converts a hook phase to a printable string
func (x HookPhase) String() string { return string(x) }

// hookExecutionJSON is used for custom JSON marshaling/unmarshaling
type hookExecutionJSON struct {
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Phase       HookPhase  `json:"phase"`
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It handles empty string time fields by treating them as zero values.
func (h *HookExecution) UnmarshalJSON(data []byte) error {
	// First try to unmarshal into a map to handle empty string time fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Replace empty string time fields with nil
	for _, field := range []string{"started_at", "completed_at"} {
		if val, ok := raw[field]; ok {
			if str, ok := val.(string); ok && str == "" {
				raw[field] = nil
			}
		}
	}

	// Re-marshal with cleaned data
	cleaned, err := json.Marshal(raw)
	if err != nil {
		return err
	}

	// Unmarshal into temporary struct with pointer time fields
	var tmp hookExecutionJSON
	if err := json.Unmarshal(cleaned, &tmp); err != nil {
		return err
	}

	// Copy values to HookExecution struct
	if tmp.StartedAt != nil {
		h.StartedAt = *tmp.StartedAt
	}
	if tmp.CompletedAt != nil {
		h.CompletedAt = *tmp.CompletedAt
	}
	h.Phase = tmp.Phase

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
// It omits zero-value time fields from the JSON output.
func (h HookExecution) MarshalJSON() ([]byte, error) {
	tmp := hookExecutionJSON{
		Phase: h.Phase,
	}

	if !h.StartedAt.IsZero() {
		tmp.StartedAt = &h.StartedAt
	}
	if !h.CompletedAt.IsZero() {
		tmp.CompletedAt = &h.CompletedAt
	}

	return json.Marshal(tmp)
}
