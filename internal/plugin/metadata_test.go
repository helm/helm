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
		PlatformCommands: []PlatformCommand{},
		PlatformHooks:    map[string][]PlatformCommand{},
	}

	// A mock plugin with legacy commands
	mockLegacyCommand := mockSubprocessCLIPlugin(t, "foo")
	mockLegacyCommand.metadata.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommands: []PlatformCommand{},
		Command:          "echo \"mock plugin\"",
		PlatformHooks:    map[string][]PlatformCommand{},
		Hooks: map[string]string{
			Install: "echo installing...",
		},
	}

	// A mock plugin with a command also set
	mockWithCommand := mockSubprocessCLIPlugin(t, "foo")
	mockWithCommand.metadata.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommands: []PlatformCommand{
			{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"mock plugin\""}},
		},
		Command: "echo \"mock plugin\"",
	}

	// A mock plugin with a hooks also set
	mockWithHooks := mockSubprocessCLIPlugin(t, "foo")
	mockWithHooks.metadata.RuntimeConfig = &RuntimeConfigSubprocess{
		PlatformCommands: []PlatformCommand{
			{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"mock plugin\""}},
		},
		PlatformHooks: map[string][]PlatformCommand{
			Install: {
				{OperatingSystem: "linux", Architecture: "", Command: "sh", Args: []string{"-c", "echo \"installing...\""}},
			},
		},
		Hooks: map[string]string{
			Install: "echo installing...",
		},
	}

	for i, item := range []struct {
		pass      bool
		plug      Plugin
		errString string
	}{
		{true, mockSubprocessCLIPlugin(t, "abcdefghijklmnopqrstuvwxyz0123456789_-ABC"), ""},
		{true, mockSubprocessCLIPlugin(t, "foo-bar-FOO-BAR_1234"), ""},
		{false, mockSubprocessCLIPlugin(t, "foo -bar"), "invalid name"},
		{false, mockSubprocessCLIPlugin(t, "$foo -bar"), "invalid name"}, // Test leading chars
		{false, mockSubprocessCLIPlugin(t, "foo -bar "), "invalid name"}, // Test trailing chars
		{false, mockSubprocessCLIPlugin(t, "foo\nbar"), "invalid name"},  // Test newline
		{true, mockNoCommand, ""},     // Test no command metadata works
		{true, mockLegacyCommand, ""}, // Test legacy command metadata works
		{false, mockWithCommand, "runtime config validation failed: both platformCommand and command are set"}, // Test platformCommand and command both set fails
		{false, mockWithHooks, "runtime config validation failed: both platformHooks and hooks are set"},       // Test platformHooks and hooks both set fails
	} {
		err := item.plug.Metadata().Validate()
		if item.pass && err != nil {
			t.Errorf("failed to validate case %d: %s", i, err)
		} else if !item.pass && err == nil {
			t.Errorf("expected case %d to fail", i)
		}
		if !item.pass && err.Error() != item.errString {
			t.Errorf("index [%d]: expected the following error: %s, but got: %s", i, item.errString, err.Error())
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
		"invalid name",
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
