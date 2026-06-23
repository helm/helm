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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func verify(t *testing.T, actual reference, registry, repository, tag, digest string) {
	t.Helper()
	assert.Equal(t, registry, actual.orasReference.Registry, "Oras reference registry")
	assert.Equal(t, repository, actual.orasReference.Repository, "Oras reference repository")
	assert.Equal(t, tag, actual.orasReference.Reference, "Oras reference reference")
	assert.Equal(t, registry, actual.Registry, "Registry")
	assert.Equal(t, repository, actual.Repository, "Repository")
	assert.Equal(t, tag, actual.Tag, "Tag")
	assert.Equal(t, digest, actual.Digest, "Digest")
	expectedString := registry
	if repository != "" {
		expectedString = expectedString + "/" + repository
	}
	if tag != "" {
		expectedString = expectedString + ":" + tag
	} else {
		expectedString = expectedString + "@" + digest
	}
	assert.Equal(t, expectedString, actual.String(), "String")
}

func TestNewReference(t *testing.T) {
	actual, err := newReference("registry.example.com/repository:1.0@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	assert.NoError(t, err)
	verify(t, actual, "registry.example.com", "repository", "1.0", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")

	actual, err = newReference("oci://registry.example.com/repository:1.0@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	assert.NoError(t, err)
	verify(t, actual, "registry.example.com", "repository", "1.0", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")

	actual, err = newReference("a/b:1@c")
	assert.NoError(t, err)
	verify(t, actual, "a", "b", "1", "c")

	actual, err = newReference("a/b:@")
	assert.NoError(t, err)
	verify(t, actual, "a", "b", "", "")

	actual, err = newReference("registry.example.com/repository:1.0+001")
	assert.NoError(t, err)
	verify(t, actual, "registry.example.com", "repository", "1.0_001", "")

	actual, err = newReference("thing:1.0")
	assert.Error(t, err)
	verify(t, actual, "", "", "", "")

	actual, err = newReference("registry.example.com/the/repository@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	assert.NoError(t, err)
	verify(t, actual, "registry.example.com", "the/repository", "", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
}
