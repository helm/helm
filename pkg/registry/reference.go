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

package registry // import "k8s.io/helm/pkg/registry"

import (
	"errors"
	"regexp"
	"strings"

	"github.com/containerd/containerd/reference"
)

var (
	validPortRegEx     = regexp.MustCompile("^([1-9]\\d{0,3}|0|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$") // adapted from https://stackoverflow.com/a/12968117
	tooManyColonsError = errors.New("ref may only contain a single colon character (:) unless specifying a port number")
)

type (
	// Reference defines the main components of a reference specification
	Reference struct {
		*reference.Spec
	}
)

// ParseReference converts a string to a Reference
func ParseReference(s string) (*Reference, error) {
	spec, err := reference.Parse(s)
	if err != nil {
		return nil, err
	}

	// convert to our custom type and make necessary mods
	ref := Reference{&spec}
	ref.fix()

	// ensure the reference is valid
	err = ref.validate()
	if err != nil {
		return nil, err
	}

	return &ref, nil
}

// fix modifies and augments a ref that may not have been parsed properly
func (ref *Reference) fix() {
	ref.fixNoTag()
	ref.fixNoLocator()
}

// fixNoTag is a fix for ref strings such as "mychart:1.0.0", which result in missing tag
func (ref *Reference) fixNoTag() {
	if ref.Object == "" {
		parts := strings.Split(ref.Locator, ":")
		numParts := len(parts)
		if 0 < numParts {
			lastIndex := numParts - 1
			lastPart := parts[lastIndex]
			if !strings.Contains(lastPart, "/") {
				ref.Locator = strings.Join(parts[:lastIndex], ":")
				ref.Object = lastPart
			}
		}
	}
}

// fixNoLocator is a fix for ref strings such as "mychart", which have the locator swapped with tag
func (ref *Reference) fixNoLocator() {
	if ref.Locator == "" {
		ref.Locator = ref.Object
		ref.Object = ""
	}
}

// validate makes sure the ref meets our criteria
func (ref *Reference) validate() error {
	return ref.validateColons()
}

// validateColons verifies the ref only contains one colon max
// (or two, there might be a port number specified i.e. :5000)
func (ref *Reference) validateColons() error {
	if strings.Contains(ref.Object, ":") {
		return tooManyColonsError
	}
	locParts := strings.Split(ref.Locator, ":")
	locLastIndex := len(locParts) - 1
	if 1 < locLastIndex {
		return tooManyColonsError
	}
	if 0 < locLastIndex {
		port := strings.Split(locParts[locLastIndex], "/")[0]
		if !isValidPort(port) {
			return tooManyColonsError
		}
	}
	return nil
}

// isValidPort returns whether or not a string looks like a valid port
func isValidPort(s string) bool {
	return validPortRegEx.MatchString(s)
}
