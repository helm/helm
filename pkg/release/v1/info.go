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

	"helm.sh/helm/v4/pkg/release/common"

	"k8s.io/apimachinery/pkg/runtime"
)

// Info describes release information.
type Info struct {
	// FirstDeployed is when the release was first deployed.
	FirstDeployed time.Time `json:"first_deployed,omitzero"`
	// LastDeployed is when the release was last deployed.
	LastDeployed time.Time `json:"last_deployed,omitzero"`
	// Deleted tracks when this object was deleted.
	Deleted time.Time `json:"deleted,omitzero"`
	// Description is human-friendly "log entry" about this release.
	Description string `json:"description,omitempty"`
	// Status is the current state of the release
	Status common.Status `json:"status,omitempty"`
	// Contains the rendered templates/NOTES.txt if available
	Notes string `json:"notes,omitempty"`
	// Contains the deployed resources information
	Resources map[string][]runtime.Object `json:"resources,omitempty"`
}

// infoJSON is used for custom JSON marshaling/unmarshaling
type infoJSON struct {
	FirstDeployed *time.Time                  `json:"first_deployed,omitempty"`
	LastDeployed  *time.Time                  `json:"last_deployed,omitempty"`
	Deleted       *time.Time                  `json:"deleted,omitempty"`
	Description   string                      `json:"description,omitempty"`
	Status        common.Status               `json:"status,omitempty"`
	Notes         string                      `json:"notes,omitempty"`
	Resources     map[string][]runtime.Object `json:"resources,omitempty"`
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It handles empty string time fields by treating them as zero values.
func (i *Info) UnmarshalJSON(data []byte) error {
	// First try to unmarshal into a map to handle empty string time fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Replace empty string time fields with nil
	for _, field := range []string{"first_deployed", "last_deployed", "deleted"} {
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
	var tmp infoJSON
	if err := json.Unmarshal(cleaned, &tmp); err != nil {
		return err
	}

	// Copy values to Info struct
	if tmp.FirstDeployed != nil {
		i.FirstDeployed = *tmp.FirstDeployed
	}
	if tmp.LastDeployed != nil {
		i.LastDeployed = *tmp.LastDeployed
	}
	if tmp.Deleted != nil {
		i.Deleted = *tmp.Deleted
	}
	i.Description = tmp.Description
	i.Status = tmp.Status
	i.Notes = tmp.Notes
	i.Resources = tmp.Resources

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
// It omits zero-value time fields from the JSON output.
func (i Info) MarshalJSON() ([]byte, error) {
	tmp := infoJSON{
		Description: i.Description,
		Status:      i.Status,
		Notes:       i.Notes,
		Resources:   i.Resources,
	}

	if !i.FirstDeployed.IsZero() {
		tmp.FirstDeployed = &i.FirstDeployed
	}
	if !i.LastDeployed.IsZero() {
		tmp.LastDeployed = &i.LastDeployed
	}
	if !i.Deleted.IsZero() {
		tmp.Deleted = &i.Deleted
	}

	return json.Marshal(tmp)
}
