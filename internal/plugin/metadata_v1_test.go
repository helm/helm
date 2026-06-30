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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataV1ValidateVersion(t *testing.T) {
	base := func() MetadataV1 {
		return MetadataV1{
			APIVersion: "v1",
			Name:       "myplugin",
			Type:       "cli/v1",
			Runtime:    "subprocess",
			Version:    "1.0.0",
		}
	}

	testsValid := map[string]string{
		"simple version":  "1.0.0",
		"with prerelease": "1.2.3-alpha.1",
		"with build meta": "1.2.3+build.123",
		"full prerelease": "1.2.3-alpha.1+build.123",
	}

	for name, version := range testsValid {
		t.Run("valid/"+name, func(t *testing.T) {
			m := base()
			m.Version = version
			assert.NoError(t, m.Validate())
		})
	}

	testsInvalid := map[string]struct {
		version string
		errMsg  string
	}{
		"empty version": {
			version: "",
			errMsg:  "plugin `version` is required",
		},
		"v prefix": {
			version: "v1.0.0",
			errMsg:  "invalid plugin `version` \"v1.0.0\": must be valid semver",
		},
		"path traversal": {
			version: "../../../../tmp/evil",
			errMsg:  "invalid plugin `version`",
		},
		"path traversal etc": {
			version: "../../../etc/passwd",
			errMsg:  "invalid plugin `version`",
		},
		"not a version": {
			version: "not-a-version",
			errMsg:  "invalid plugin `version`",
		},
	}

	for name, tc := range testsInvalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			m := base()
			m.Version = tc.version
			err := m.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}
