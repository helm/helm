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

import "testing"

func verify(t *testing.T, actual reference, registry, repository, tag, digest string) {
	if registry != actual.orasReference.Registry {
		t.Errorf("Oras reference registry expected %v actual %v", registry, actual.Registry)
	}
	if repository != actual.orasReference.Repository {
		t.Errorf("Oras reference repository expected %v actual %v", repository, actual.Repository)
	}
	if tag != actual.orasReference.Reference {
		t.Errorf("Oras reference reference expected %v actual %v", tag, actual.Tag)
	}
	if registry != actual.Registry {
		t.Errorf("Registry expected %v actual %v", registry, actual.Registry)
	}
	if repository != actual.Repository {
		t.Errorf("Repository expected %v actual %v", repository, actual.Repository)
	}
	if tag != actual.Tag {
		t.Errorf("Tag expected %v actual %v", tag, actual.Tag)
	}
	if digest != actual.Digest {
		t.Errorf("Digest expected %v actual %v", digest, actual.Digest)
	}
	expectedString := registry
	if repository != "" {
		expectedString = expectedString + "/" + repository
	}
	if tag != "" {
		expectedString = expectedString + ":" + tag
	} else {
		expectedString = expectedString + "@" + digest
	}
	if actual.String() != expectedString {
		t.Errorf("String expected %s actual %s", expectedString, actual.String())
	}
}

func TestNewReference(t *testing.T) {
	actual, err := newReference("registry.example.com/repository:1.0@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "repository", "1.0", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")

	actual, err = newReference("oci://registry.example.com/repository:1.0@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "repository", "1.0", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")

	actual, err = newReference("a/b:1@c")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "a", "b", "1", "c")

	actual, err = newReference("a/b:@")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "a", "b", "", "")

	actual, err = newReference("registry.example.com/repository:1.0+001")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "repository", "1.0_001", "")

	actual, err = newReference("thing:1.0")
	if err == nil {
		t.Errorf("Expect error error %v", err)
	}
	verify(t, actual, "", "", "", "")

	actual, err = newReference("registry.example.com/the/repository@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "the/repository", "", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
}
