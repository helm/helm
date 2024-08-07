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

func verify(t *testing.T, actual Reference, registry, repository, tag, digest string) {
	if registry != actual.OrasReference.Registry {
		t.Errorf("Oras Reference registry expected %v actual %v", registry, actual.Registry)
	}
	if repository != actual.OrasReference.Repository {
		t.Errorf("Oras Reference repository expected %v actual %v", repository, actual.Repository)
	}
	if tag != actual.OrasReference.Reference {
		t.Errorf("Oras Reference reference expected %v actual %v", tag, actual.Tag)
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
}

func TestNewReference(t *testing.T) {
	actual, err := NewReference("registry.example.com/repository:1.0@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "repository", "1.0", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")

	actual, err = NewReference("oci://registry.example.com/repository:1.0@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "repository", "1.0", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")

	actual, err = NewReference("a/b:1@c")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "a", "b", "1", "c")

	actual, err = NewReference("a/b:@")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "a", "b", "", "")

	actual, err = NewReference("registry.example.com/repository:1.0+001")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "repository", "1.0_001", "")

	actual, err = NewReference("thing:1.0")
	if err == nil {
		t.Errorf("Expect error error %v", err)
	}
	verify(t, actual, "", "", "", "")

	actual, err = NewReference("registry.example.com/the/repository@sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	verify(t, actual, "registry.example.com", "the/repository", "", "sha256:c6841b3a895f1444a6738b5d04564a57e860ce42f8519c3be807fb6d9bee7888")
}
