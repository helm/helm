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

package chartutil

import (
	"fmt"
	"regexp"

	"github.com/pkg/errors"
)

// validName is a regular expression for resource names.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
//
// This follows the above regular expression (but requires a full string match, not partial).
//
// The Kubernetes documentation is here, though it is not entirely correct:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
var validName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// validchartName is a regular expression for testing the supplied name of a chart.
// This regular expression is probably stricter than it needs to be. We can relax it
// somewhat. Newline characters, as well as $, quotes, +, parens, and % are known to be
// problematic.
var validchartName = regexp.MustCompile("^[a-zA-Z0-9._-]+$")

var (
	// errMissingName indicates that a release (name) was not provided.
	errMissingName = errors.New("no name provided")

	// errInvalidName indicates that an invalid release name was provided
	errInvalidName = errors.New(fmt.Sprintf(
		"invalid release name, must match regex %s and the length must not be longer than 53",
		validName.String()))

	// errInvalidKubernetesName indicates that the name does not meet the Kubernetes
	// restrictions on metadata names.
	errInvalidKubernetesName = errors.New(fmt.Sprintf(
		"invalid metadata name, must match regex %s and the length must not be longer than 253",
		validName.String()))
)

const (
	// maxNameLen is the maximum length Helm allows for a release name
	maxReleaseNameLen = 53
	// maxMetadataNameLen is the maximum length Kubernetes allows for any name.
	maxMetadataNameLen = 253

	// maxChartNameLength is lower than the limits we know of with certain file systems,
	// and with certain Kubernetes fields.
	maxChartNameLength = 250
)

// ValidateReleaseName performs checks for an entry for a Helm release name
//
// For Helm to allow a name, it must be below a certain character count (53) and also match
// a reguar expression.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
//
// This follows the above regular expression (but requires a full string match, not partial).
//
// The Kubernetes documentation is here, though it is not entirely correct:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
func ValidateReleaseName(name string) error {
	// This case is preserved for backwards compatibility
	if name == "" {
		return errMissingName

	}
	if len(name) > maxReleaseNameLen || !validName.MatchString(name) {
		return errInvalidName
	}
	return nil
}

// ValidateMetadataName validates the name field of a Kubernetes metadata object.
//
// Empty strings, strings longer than 253 chars, or strings that don't match the regexp
// will fail.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
//
// This follows the above regular expression (but requires a full string match, not partial).
//
// The Kubernetes documentation is here, though it is not entirely correct:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
func ValidateMetadataName(name string) error {
	if name == "" || len(name) > maxMetadataNameLen || !validName.MatchString(name) {
		return errInvalidKubernetesName
	}
	return nil
}

// ValidateChartName validates the name of helm charts
//
// Empty strings, strings longer than 250 chars, or strings that don't match the regexp
// will fail.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	[a-zA-Z0-9._-]+$
//
// This follows the above regular expression (but requires a full string match, not partial).
func ValidateChartName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if name == "" || len(name) > maxChartNameLength {
		return fmt.Errorf("chart name must be between 1 and %d characters", maxChartNameLength)
	}
	if !validchartName.MatchString(name) {
		return fmt.Errorf("chart name must match the regular expression %q", validchartName.String())
	}
	return nil
}
