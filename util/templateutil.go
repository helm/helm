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
	"strings"
	"log"

	"github.com/kubernetes/deployment-manager/common"
)

// IsTemplate returns whether a given type is a template.
func IsTemplate(t string, imports []*common.ImportFile) bool {
	log.Printf("IsTemplate: %s : %+v", t, imports)
	for _, imp := range imports {
		log.Printf("Checking: %s", imp.Name)
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
	if !strings.HasPrefix(t, "github.com/") {
		return false
	}
	s := strings.Split(t, "/")
	if len(s) != 5 {
		return false
	}
	v := strings.Split(s[4], ":")
	if len(v) != 2 {
		return false
	}
	return true
}
