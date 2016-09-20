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

package chartutil

import (
	"errors"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

const (
	requirementsName = "requirements.yaml"
	lockfileName     = "requirements.lock"
)

var (
	// ErrRequirementsNotFound indicates that a requirements.yaml is not found.
	ErrRequirementsNotFound = errors.New(requirementsName + " not found")
	// ErrLockfileNotFound indicates that a requirements.lock is not found.
	ErrLockfileNotFound = errors.New(lockfileName + " not found")
)

// Dependency describes a chart upon which another chart depends.
//
// Dependencies can be used to express developer intent, or to capture the state
// of a chart.
type Dependency struct {
	// Name is the name of the dependency.
	//
	// This must mach the name in the dependency's Chart.yaml.
	Name string `json:"name"`
	// Version is the version (range) of this chart.
	//
	// A lock file will always produce a single version, while a dependency
	// may contain a semantic version range.
	Version string `json:"version,omitempty"`
	// The URL to the repository.
	//
	// Appending `index.yaml` to this string should result in a URL that can be
	// used to fetch the repository index.
	Repository string `json:"repository"`
}

// Requirements is a list of requirements for a chart.
//
// Requirements are charts upon which this chart depends. This expresses
// developer intent.
type Requirements struct {
	Dependencies []*Dependency `json:"dependencies"`
}

// RequirementsLock is a lock file for requirements.
//
// It represents the state that the dependencies should be in.
type RequirementsLock struct {
	// Genderated is the date the lock file was last generated.
	Generated time.Time `json:"generated"`
	// Digest is a hash of the requirements file used to generate it.
	Digest string `json:"digest"`
	// Dependencies is the list of dependencies that this lock file has locked.
	Dependencies []*Dependency `json:"dependencies"`
}

// LoadRequirements loads a requirements file from an in-memory chart.
func LoadRequirements(c *chart.Chart) (*Requirements, error) {
	var data []byte
	for _, f := range c.Files {
		if f.TypeUrl == requirementsName {
			data = f.Value
		}
	}
	if len(data) == 0 {
		return nil, ErrRequirementsNotFound
	}
	r := &Requirements{}
	return r, yaml.Unmarshal(data, r)
}

// LoadRequirementsLock loads a requirements lock file.
func LoadRequirementsLock(c *chart.Chart) (*RequirementsLock, error) {
	var data []byte
	for _, f := range c.Files {
		if f.TypeUrl == lockfileName {
			data = f.Value
		}
	}
	if len(data) == 0 {
		return nil, ErrLockfileNotFound
	}
	r := &RequirementsLock{}
	return r, yaml.Unmarshal(data, r)
}
