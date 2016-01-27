/*
Copyright 2015 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"github.com/kubernetes/deployment-manager/common"

	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Registry abstracts a registry that holds charts, which can be
// used in a Deployment Manager configuration. There can be multiple
// registry implementations.
type Registry interface {
	// GetRegistryName returns the name of this registry
	GetRegistryName() string
	// GetRegistryType returns the type of this registry.
	GetRegistryType() common.RegistryType
	// GetRegistryShortURL returns the short URL for this registry.
	GetRegistryShortURL() string
	// GetRegistryFormat returns the format of this registry.
	GetRegistryFormat() common.RegistryFormat

	// ListTypes lists types in this registry whose string values conform to the
	// supplied regular expression, or all types, if the regular expression is nil.
	ListTypes(regex *regexp.Regexp) ([]Type, error)
	// GetDownloadURLs returns the URLs required to download the type contents.
	GetDownloadURLs(t Type) ([]*url.URL, error)
}

// GithubRegistry abstracts a registry that resides in a Github repository.
type GithubRegistry interface {
	Registry // A GithubRegistry is a Registry.
	// GetRegistryOwner returns the owner name for this registry
	GetRegistryOwner() string
	// GetRegistryRepository returns the repository name for this registry.
	GetRegistryRepository() string
	// GetRegistryPath returns the path to the registry in the repository.
	GetRegistryPath() string
}

type Type struct {
	Collection string
	Name       string
	Version    SemVer
}

// NewType initializes a type
func NewType(collection, name, version string) (Type, error) {
	result := Type{Collection: collection, Name: name}
	err := result.SetVersion(version)
	return result, err
}

// NewTypeOrDie initializes a type and panics if initialization fails
func NewTypeOrDie(collection, name, version string) Type {
	result, err := NewType(collection, name, version)
	if err != nil {
		panic(err)
	}

	return result
}

// Type conforms to the Stringer interface.
func (t Type) String() string {
	var result string
	if t.Collection != "" {
		result = t.Collection + "/"
	}

	result = result + t.Name
	version := t.GetVersion()
	if version != "" && version != "v0" {
		result = result + ":" + version
	}

	return result
}

// GetVersion returns the type version with the letter "v" prepended.
func (t Type) GetVersion() string {
	var result string
	version := t.Version.String()
	if version != "0" {
		result = "v" + version
	}

	return result
}

// SetVersion strips the letter "v" from version, if present,
// and sets the the version of the type to the result.
func (t *Type) SetVersion(version string) error {
	vstring := strings.TrimPrefix(version, "v")
	s, err := ParseSemVer(vstring)
	if err != nil {
		return err
	}

	t.Version = s
	return nil
}

// ParseType takes a registry type string and parses it into a *registry.Type.
// TODO: needs better validation that this is actually a registry type.
func ParseType(ts string) (Type, error) {
	tt := Type{}
	tList := strings.Split(ts, ":")
	if len(tList) == 2 {
		if err := tt.SetVersion(tList[1]); err != nil {
			return tt, fmt.Errorf("malformed type string: %s", ts)
		}
	}

	cList := strings.Split(tList[0], "/")
	if len(cList) == 1 {
		tt.Name = tList[0]
	} else {
		tt.Collection = cList[0]
		tt.Name = cList[1]
	}

	return tt, nil
}
