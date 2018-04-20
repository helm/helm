/*
Copyright 2018 The Kubernetes Authors All rights reserved.
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

// Info describes release information.
type Info struct {
	Status        *Status   `json:"status,omitempty"`
	FirstDeployed time.Time `json:"first_deployed,omitempty"`
	LastDeployed  time.Time `json:"last_deployed,omitempty"`
	// Deleted tracks when this object was deleted.
	Deleted time.Time `json:"deleted,omitempty"`
	// Description is human-friendly "log entry" about this release.
	Description string `json:"Description,omitempty"`
}
