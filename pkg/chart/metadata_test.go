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

package chart

import (
	"testing"
)

func TestMetadata_Validate(t *testing.T) {
	t.Run("return error when metadata is missing", func(t *testing.T) {
		var metadata *Metadata
		err := metadata.Validate()
		if err == nil {
			t.Fatal("Expected err to be non-nil")
		}
		if err != ErrMissingMetadata {
			t.Errorf("Expected '%s', got '%s'", ErrMissingMetadata.Error(), err.Error())
		}
	})

	t.Run("return error when api version is missing", func(t *testing.T) {
		metadata := &Metadata{}
		err := metadata.Validate()
		if err == nil {
			t.Fatal("Expected err to be non-nil")
		}
		if err != ErrMissingAPIVersion {
			t.Errorf("Expected '%s', got '%s'", ErrMissingAPIVersion.Error(), err.Error())
		}
	})

	t.Run("return error when name is missing", func(t *testing.T) {
		metadata := &Metadata{
			APIVersion: APIVersionV1,
		}
		err := metadata.Validate()
		if err == nil {
			t.Fatal("Expected err to be non-nil")
		}
		if err != ErrMissingName {
			t.Errorf("Expected %s, got '%s'", ErrMissingName, err.Error())
		}
	})

	t.Run("return error when version is missing", func(t *testing.T) {
		metadata := &Metadata{
			APIVersion: APIVersionV1,
			Name:       "testChart",
		}
		err := metadata.Validate()
		if err == nil {
			t.Fatal("Expected err to be non-nil")
		}
		if err != ErrMissingVersion {
			t.Errorf("Expected %s, got '%s'", ErrMissingVersion, err.Error())
		}
	})

	t.Run("return error when type is invalid", func(t *testing.T) {
		metadata := &Metadata{
			APIVersion: APIVersionV1,
			Name:       "testChart",
			Version:    "0.1.0",
			Type:       "dummy",
		}
		err := metadata.Validate()
		if err == nil {
			t.Fatal("Expected err to be non-nil")
		}
		if err != ErrInvalidType {
			t.Errorf("Expected '%s', got '%s'", ErrInvalidType.Error(), err.Error())
		}
	})
}
