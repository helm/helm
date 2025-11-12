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
	"strings"
	"testing"
)

func TestMetadataLegacyValidate_EmptyName(t *testing.T) {
	metadata := MetadataLegacy{
		Name:    "",
		Version: "1.0.0",
	}

	err := metadata.Validate()
	if err == nil {
		t.Fatal("expected error for empty plugin name")
	}

	expectedMsg := "plugin name is empty or missing"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestMetadataLegacyValidate_InvalidName(t *testing.T) {
	metadata := MetadataLegacy{
		Name:    "invalid name",
		Version: "1.0.0",
	}

	err := metadata.Validate()
	if err == nil {
		t.Fatal("expected error for invalid plugin name")
	}

	expectedMsg := "plugin names can only contain ASCII characters"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain %q, got %q", expectedMsg, err.Error())
	}
}
