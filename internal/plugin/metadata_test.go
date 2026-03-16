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

func TestValidatePluginData(t *testing.T) {

	// A mock plugin with no commands
	mockNoCommand := mockSubprocessCLIPlugin(t, "foo")
	mockNoCommand.metadata.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{},
		PlatformHooks:   map[string][]PlatformCommand{},
	}

	// A mock plugin with legacy commands
	mockLegacyCommand := mockSubprocessCLIPlugin(t, "foo")
	mockLegacyCommand.metadata.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommand: []PlatformCommand{
			{
				Command: "echo \"mock plugin\"",
			},
		},
		PlatformHooks: map[string][]PlatformCommand{
			Install: {
				PlatformCommand{
					Command: "echo installing...",
				},
			},
		},
	}

	for i, item := range []struct {
		pass      bool
		plug      Plugin
		errString string
	}{
		{true, mockSubprocessCLIPlugin(t, "abcdefghijklmnopqrstuvwxyz0123456789_-ABC"), ""},
		{true, mockSubprocessCLIPlugin(t, "foo-bar-FOO-BAR_1234"), ""},
		{false, mockSubprocessCLIPlugin(t, "foo -bar"), "invalid plugin name"},
		{false, mockSubprocessCLIPlugin(t, "$foo -bar"), "invalid plugin name"}, // Test leading chars
		{false, mockSubprocessCLIPlugin(t, "foo -bar "), "invalid plugin name"}, // Test trailing chars
		{false, mockSubprocessCLIPlugin(t, "foo\nbar"), "invalid plugin name"},  // Test newline
		{true, mockNoCommand, ""},     // Test no command metadata works
		{true, mockLegacyCommand, ""}, // Test legacy command metadata works
	} {
		err := item.plug.Metadata().Validate()
		if item.pass && err != nil {
			t.Errorf("failed to validate case %d: %s", i, err)
		} else if !item.pass && err == nil {
			t.Errorf("expected case %d to fail", i)
		}
		if !item.pass && !strings.Contains(err.Error(), item.errString) {
			t.Errorf("index [%d]: expected error to contain: %s, but got: %s", i, item.errString, err.Error())
		}
	}
}

func TestMetadataValidateMultipleErrors(t *testing.T) {
	// Create metadata with multiple validation issues
	metadata := Metadata{
		Name:          "invalid name with spaces", // Invalid name
		APIVersion:    "",                         // Empty API version
		Type:          "",                         // Empty type
		Runtime:       "",                         // Empty runtime
		Config:        nil,                        // Missing config
		RuntimeConfig: nil,                        // Missing runtime config
	}

	err := metadata.Validate()
	if err == nil {
		t.Fatal("expected validation to fail with multiple errors")
	}

	errStr := err.Error()

	// Check that all expected errors are present in the joined error
	expectedErrors := []string{
		"invalid plugin name",
		"empty APIVersion",
		"empty type field",
		"empty runtime field",
		"missing config field",
		"missing runtimeConfig field",
	}

	for _, expectedErr := range expectedErrors {
		if !strings.Contains(errStr, expectedErr) {
			t.Errorf("expected error to contain %q, but got: %v", expectedErr, errStr)
		}
	}

	// Verify that the error contains the correct number of error messages
	errorCount := 0
	for _, expectedErr := range expectedErrors {
		if strings.Contains(errStr, expectedErr) {
			errorCount++
		}
	}

	if errorCount < len(expectedErrors) {
		t.Errorf("expected %d errors, but only found %d in: %v", len(expectedErrors), errorCount, errStr)
	}
}
