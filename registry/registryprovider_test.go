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

package registry

import (
	"testing"
)

func testUrlConversionDriver(rp RegistryProvider, tests map[string]TestURLAndError, t *testing.T) {
	for in, expected := range tests {
		// TODO(vaikas): Test to make sure it's the right registry.
		actual, _, err := GetDownloadURLs(rp, in)
		if err != expected.Err {
			t.Fatalf("failed on: %s : expected error %v but got %v", in, expected.Err, err)
		}

		if actual[0] != expected.URL {
			t.Fatalf("failed on: %s : expected %s but got %v", in, expected.URL, actual)
		}
	}
}

func TestShortGithubUrlTemplateMapping(t *testing.T) {
	githubUrlMaps := map[Type]TestURLAndError{
		NewTypeOrDie("common", "replicatedservice", "v1"): TestURLAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		NewTypeOrDie("storage", "redis", "v1"):            TestURLAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/storage/redis/v1/redis.jinja", nil},
	}

	tests := map[string]TestURLAndError{
		"github.com/kubernetes/application-dm-templates/common/replicatedservice:v1": TestURLAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/common/replicatedservice/v1/replicatedservice.py", nil},
		"github.com/kubernetes/application-dm-templates/storage/redis:v1":            TestURLAndError{"https://raw.githubusercontent.com/kubernetes/application-dm-templates/master/storage/redis/v1/redis.jinja", nil},
	}

	grp := NewTestGithubRegistryProvider("github.com/kubernetes/application-dm-templates", githubUrlMaps)
	// TODO(vaikas): XXXX FIXME Add gcsrp
	testUrlConversionDriver(NewRegistryProvider(nil, grp, nil, NewInmemCredentialProvider()), tests, t)
}

func TestShortGithubUrlPackageMapping(t *testing.T) {
	githubUrlMaps := map[Type]TestURLAndError{
		NewTypeOrDie("", "mongodb", ""): TestURLAndError{"https://raw.githubusercontent.com/helm/charts/master/mongodb/manifests/mongodb.yaml", nil},
		NewTypeOrDie("", "redis", ""):   TestURLAndError{"https://raw.githubusercontent.com/helm/charts/master/redis/manifests/redis.yaml", nil},
	}

	tests := map[string]TestURLAndError{
		"github.com/helm/charts/mongodb": TestURLAndError{"https://raw.githubusercontent.com/helm/charts/master/mongodb/manifests/mongodb.yaml", nil},
		"github.com/helm/charts/redis":   TestURLAndError{"https://raw.githubusercontent.com/helm/charts/master/redis/manifests/redis.yaml", nil},
	}

	grp := NewTestGithubRegistryProvider("github.com/helm/charts", githubUrlMaps)
	// TODO(vaikas): XXXX FIXME Add gcsrp
	testUrlConversionDriver(NewRegistryProvider(nil, grp, nil, NewInmemCredentialProvider()), tests, t)
}
