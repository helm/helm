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

package registry // import "helm.sh/helm/v3/internal/experimental/registry"

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	validPortRegEx = regexp.MustCompile(`^([1-9]\d{0,3}|0|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$`) // adapted from https://stackoverflow.com/a/12968117
	// TODO: Currently we don't support digests, so we are only splitting on the
	// colon. However, when we add support for digests, we'll need to use the
	// regexp anyway to split on both colons and @, so leaving it like this for
	// now
	referenceDelimiter = regexp.MustCompile(`[:]`)
	errEmptyRepo       = errors.New("parsed repo was empty")
	errTooManyColons   = errors.New("ref may only contain a single colon character (:) unless specifying a port number")
)

type (
	// Reference defines the main components of a reference specification
	Reference struct {
		Tag  string
		Repo string
	}
)

// ParseReference converts a string to a Reference
func ParseReference(s string) (*Reference, error) {
	if s == "" {
		return nil, errEmptyRepo
	}
	// Split the components of the string on the colon or @, if it is more than 3,
	// immediately return an error. Other validation will be performed later in
	// the function
	splitComponents := fixSplitComponents(referenceDelimiter.Split(s, -1))
	if len(splitComponents) > 3 {
		return nil, errTooManyColons
	}

	var ref *Reference
	switch len(splitComponents) {
	case 1:
		ref = &Reference{Repo: splitComponents[0]}
	case 2:
		ref = &Reference{Repo: splitComponents[0], Tag: splitComponents[1]}
	case 3:
		ref = &Reference{Repo: strings.Join(splitComponents[:2], ":"), Tag: splitComponents[2]}
	}

	// ensure the reference is valid
	err := ref.validate()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// FullName the full name of a reference (repo:tag)
func (ref *Reference) FullName() string {
	if ref.Tag == "" {
		return ref.Repo
	}
	return fmt.Sprintf("%s:%s", ref.Repo, ref.Tag)
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
	// Makes sure the repo results in a parsable URL (similar to what is done
	// with containerd reference parsing)
	_, err := url.Parse("//" + ref.Repo)
	return err
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

// fixSplitComponents this will modify reference parts based on presence of port
// Example: {localhost, 5000/x/y/z, 0.1.0} => {localhost:5000/x/y/z, 0.1.0}
func fixSplitComponents(c []string) []string {
	if len(c) <= 1 {
		return c
	}
	possiblePortParts := strings.Split(c[1], "/")
	if _, err := strconv.Atoi(possiblePortParts[0]); err == nil {
		components := []string{strings.Join(c[:2], ":")}
		components = append(components, c[2:]...)
		return components
	}
	return c
}
