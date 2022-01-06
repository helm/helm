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
	// According to the Kubernetes docs (https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#rfc-1035-label-names)
	// some resource names have a max length of 63 characters while others have a max
	// length of 253 characters. As we cannot be sure the resources used in a chart, we
	// therefore need to limit it to 63 chars and reserve 10 chars for additional part to name
	// of the resource. The reason is that chart maintainers can use release name as part of
	// the resource name (and some additional chars).
	maxReleaseNameLen = 53
	// maxMetadataNameLen is the maximum length Kubernetes allows for any name.
	maxMetadataNameLen = 253
)

// ValidateReleaseName performs checks for an entry for a Helm release name
//
// For Helm to allow a name, it must be below a certain character count (53) and also match
// a regular expression.
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
//
// Deprecated: remove in Helm 4.  Name validation now uses rules defined in
// pkg/lint/rules.validateMetadataNameFunc()
func ValidateMetadataName(name string) error {
	if name == "" || len(name) > maxMetadataNameLen || !validName.MatchString(name) {
		return errInvalidKubernetesName
	}
	return nil
}
