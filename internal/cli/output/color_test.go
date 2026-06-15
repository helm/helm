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

package output

import (
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/release/common"
)

func TestColorizeStatus(t *testing.T) {

	tests := []struct {
		name       string
		status     common.Status
		noColor    bool
		envNoColor string
		wantColor  bool // whether we expect color codes in output
	}{
		{
			name:       "deployed status with color",
			status:     common.StatusDeployed,
			noColor:    false,
			envNoColor: "",
			wantColor:  true,
		},
		{
			name:       "deployed status without color flag",
			status:     common.StatusDeployed,
			noColor:    true,
			envNoColor: "",
			wantColor:  false,
		},
		{
			name:       "deployed status with NO_COLOR env",
			status:     common.StatusDeployed,
			noColor:    false,
			envNoColor: "1",
			wantColor:  false,
		},
		{
			name:       "failed status with color",
			status:     common.StatusFailed,
			noColor:    false,
			envNoColor: "",
			wantColor:  true,
		},
		{
			name:       "pending install status with color",
			status:     common.StatusPendingInstall,
			noColor:    false,
			envNoColor: "",
			wantColor:  true,
		},
		{
			name:       "unknown status with color",
			status:     common.StatusUnknown,
			noColor:    false,
			envNoColor: "",
			wantColor:  true,
		},
		{
			name:       "superseded status with color",
			status:     common.StatusSuperseded,
			noColor:    false,
			envNoColor: "",
			wantColor:  false, // superseded doesn't get colored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.envNoColor)

			result := ColorizeStatus(tt.status, tt.noColor)

			// Check if result contains ANSI escape codes
			hasColor := strings.Contains(result, "\033[")

			// In test environment, term.IsTerminal will be false, so we won't get color
			// unless we're testing the logic without terminal detection
			if hasColor && !tt.wantColor {
				t.Errorf("ColorizeStatus() returned color when none expected: %q", result)
			}

			// Always check the status text is present
			if !strings.Contains(result, tt.status.String()) {
				t.Errorf("ColorizeStatus() = %q, want to contain %q", result, tt.status.String())
			}
		})
	}
}

func TestColorizeHeader(t *testing.T) {

	tests := []struct {
		name       string
		header     string
		noColor    bool
		envNoColor string
	}{
		{
			name:       "header with color",
			header:     "NAME",
			noColor:    false,
			envNoColor: "",
		},
		{
			name:       "header without color flag",
			header:     "NAME",
			noColor:    true,
			envNoColor: "",
		},
		{
			name:       "header with NO_COLOR env",
			header:     "NAME",
			noColor:    false,
			envNoColor: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.envNoColor)

			result := ColorizeHeader(tt.header, tt.noColor)

			// Always check the header text is present
			if !strings.Contains(result, tt.header) {
				t.Errorf("ColorizeHeader() = %q, want to contain %q", result, tt.header)
			}
		})
	}
}

func TestColorizeNamespace(t *testing.T) {

	tests := []struct {
		name       string
		namespace  string
		noColor    bool
		envNoColor string
	}{
		{
			name:       "namespace with color",
			namespace:  "default",
			noColor:    false,
			envNoColor: "",
		},
		{
			name:       "namespace without color flag",
			namespace:  "default",
			noColor:    true,
			envNoColor: "",
		},
		{
			name:       "namespace with NO_COLOR env",
			namespace:  "default",
			noColor:    false,
			envNoColor: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.envNoColor)

			result := ColorizeNamespace(tt.namespace, tt.noColor)

			// Always check the namespace text is present
			if !strings.Contains(result, tt.namespace) {
				t.Errorf("ColorizeNamespace() = %q, want to contain %q", result, tt.namespace)
			}
		})
	}
}
