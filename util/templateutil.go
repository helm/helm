/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package util

import (
	"regexp"

	"github.com/kubernetes/deployment-manager/common"
)

var TemplateRegistryMatcher = regexp.MustCompile("github.com/(.*)/(.*)/(.*)/(.*):(.*)")

// RE for Registry that does not support versions and can have multiple files without imports.
var PackageRegistryMatcher = regexp.MustCompile("github.com/(.*)/(.*)/(.*)")

// IsTemplate returns whether a given type is a template.
func IsTemplate(t string, imports []*common.ImportFile) bool {
	for _, imp := range imports {
		if imp.Name == t {
			return true
		}
	}
	return false
}

// IsGithubShortType returns whether a given type is a type description in a short format to a github repository type.
// For now, this means using github types:
// github.com/owner/repo/qualifier/type:version
// for example:
// github.com/kubernetes/application-dm-templates/storage/redis:v1
func IsGithubShortType(t string) bool {
	return TemplateRegistryMatcher.MatchString(t)
}

// IsGithubShortPackageType returns whether a given type is a type description in a short format to a github
// package repository type.
// For now, this means using github types:
// github.com/owner/repo/type
// for example:
// github.com/helm/charts/cassandra
func IsGithubShortPackageType(t string) bool {
	return PackageRegistryMatcher.MatchString(t)
}
