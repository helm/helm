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

package registry // import "helm.sh/helm/internal/experimental/registry"

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/containerd/containerd/reference"
)

var (
	validPortRegEx   = regexp.MustCompile(`^([1-9]\d{0,3}|0|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$`) // adapted from https://stackoverflow.com/a/12968117
	errEmptyRepo     = errors.New("parsed repo was empty")
	errTooManyColons = errors.New("ref may only contain a single colon character (:) unless specifying a port number")
)

type (
	// Reference defines the main components of a reference specification
	Reference struct {
		*reference.Spec
		Tag  string
		Repo string
	}
)

// ParseReference converts a string to a Reference
func ParseReference(s string) (*Reference, error) {
	spec, err := reference.Parse(s)
	if err != nil {
		return nil, err
	}

	// convert to our custom type and make necessary mods
	ref := Reference{Spec: &spec}
	ref.setExtraFields()

	// ensure the reference is valid
	err = ref.validate()
	if err != nil {
		return nil, err
	}

	return &ref, nil
}

// FullName the full name of a reference (repo:tag)
func (ref *Reference) FullName() string {
	if ref.Tag == "" {
		return ref.Repo
	}
	return fmt.Sprintf("%s:%s", ref.Repo, ref.Tag)
}

// setExtraFields adds the Repo and Tag fields to a Reference
func (ref *Reference) setExtraFields() {
	ref.Tag = ref.Object
	ref.Repo = ref.Locator
	ref.fixNoTag()
	ref.fixNoRepo()
}

// fixNoTag is a fix for ref strings such as "mychart:1.0.0", which result in missing tag
func (ref *Reference) fixNoTag() {
	if ref.Tag == "" {
		parts := strings.Split(ref.Repo, ":")
		numParts := len(parts)
		if 0 < numParts {
			lastIndex := numParts - 1
			lastPart := parts[lastIndex]
			if !strings.Contains(lastPart, "/") {
				ref.Repo = strings.Join(parts[:lastIndex], ":")
				ref.Tag = lastPart
			}
		}
	}
}

// fixNoRepo is a fix for ref strings such as "mychart", which have the repo swapped with tag
func (ref *Reference) fixNoRepo() {
	if ref.Repo == "" {
		ref.Repo = ref.Tag
		ref.Tag = ""
	}
}

// validate makes sure the ref meets our criteria
func (ref *Reference) validate() error {
	err := ref.validateRepo()
	if err != nil {
		return err
	}
	return ref.validateNumColons()
}

// validateRepo checks that the Repo field is non-empty
func (ref *Reference) validateRepo() error {
	if ref.Repo == "" {
		return errEmptyRepo
	}
	return nil
}

// validateNumColon ensures the ref only contains a single colon character (:)
// (or potentially two, there might be a port number specified i.e. :5000)
func (ref *Reference) validateNumColons() error {
	if strings.Contains(ref.Tag, ":") {
		return errTooManyColons
	}
	parts := strings.Split(ref.Repo, ":")
	lastIndex := len(parts) - 1
	if 1 < lastIndex {
		return errTooManyColons
	}
	if 0 < lastIndex {
		port := strings.Split(parts[lastIndex], "/")[0]
		if !isValidPort(port) {
			return errTooManyColons
		}
	}
	return nil
}

// isValidPort returns whether or not a string looks like a valid port
func isValidPort(s string) bool {
	return validPortRegEx.MatchString(s)
}
