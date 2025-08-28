//go:build !windows

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

package pusher

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/registry"
)

func TestNewOCIPusher(t *testing.T) {
	p, err := NewOCIPusher()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := p.(*OCIPusher); !ok {
		t.Fatal("Expected NewOCIPusher to produce an *OCIPusher")
	}

	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")
	insecureSkipTLSverify := false
	plainHTTP := false

	// Test with options
	p, err = NewOCIPusher(
		WithTLSClientConfig(pub, priv, ca),
		WithInsecureSkipTLSVerify(insecureSkipTLSverify),
		WithPlainHTTP(plainHTTP),
	)
	if err != nil {
		t.Fatal(err)
	}

	op, ok := p.(*OCIPusher)
	if !ok {
		t.Fatal("Expected NewOCIPusher to produce an *OCIPusher")
	}

	if op.opts.certFile != pub {
		t.Errorf("Expected NewOCIPusher to contain %q as the public key file, got %q", pub, op.opts.certFile)
	}

	if op.opts.keyFile != priv {
		t.Errorf("Expected NewOCIPusher to contain %q as the private key file, got %q", priv, op.opts.keyFile)
	}

	if op.opts.caFile != ca {
		t.Errorf("Expected NewOCIPusher to contain %q as the CA file, got %q", ca, op.opts.caFile)
	}

	if op.opts.plainHTTP != plainHTTP {
		t.Errorf("Expected NewOCIPusher to have plainHTTP as %t, got %t", plainHTTP, op.opts.plainHTTP)
	}

	if op.opts.insecureSkipTLSverify != insecureSkipTLSverify {
		t.Errorf("Expected NewOCIPusher to have insecureSkipVerifyTLS as %t, got %t", insecureSkipTLSverify, op.opts.insecureSkipTLSverify)
	}

	// Test if setting registryClient is being passed to the ops
	registryClient, err := registry.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	p, err = NewOCIPusher(
		WithRegistryClient(registryClient),
	)
	if err != nil {
		t.Fatal(err)
	}
	op, ok = p.(*OCIPusher)
	if !ok {
		t.Fatal("expected NewOCIPusher to produce an *OCIPusher")
	}

	if op.opts.registryClient != registryClient {
		t.Errorf("Expected NewOCIPusher to contain %p as RegistryClient, got %p", registryClient, op.opts.registryClient)
	}
}

func TestOCIPusher_Push_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		chartRef      string
		expectedError string
		setupFunc     func() string
	}{
		{
			name:          "non-existent file",
			chartRef:      "/non/existent/file.tgz",
			expectedError: "no such file",
		},
		{
			name:          "directory instead of file",
			expectedError: "cannot push directory, must provide chart archive (.tgz)",
			setupFunc: func() string {
				tempDir := t.TempDir()
				return tempDir
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pusher, err := NewOCIPusher()
			if err != nil {
				t.Fatal(err)
			}

			chartRef := tt.chartRef
			if tt.setupFunc != nil {
				chartRef = tt.setupFunc()
			}

			err = pusher.Push(chartRef, "oci://localhost:5000/test")
			if err == nil {
				t.Fatal("Expected error but got none")
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing %q, got %q", tt.expectedError, err.Error())
			}
		})
	}
}

func TestOCIPusher_newRegistryClient(t *testing.T) {
	cd := "../../testdata"
	join := filepath.Join
	ca, pub, priv := join(cd, "rootca.crt"), join(cd, "crt.pem"), join(cd, "key.pem")

	tests := []struct {
		name          string
		opts          []Option
		expectError   bool
		errorContains string
	}{
		{
			name: "plain HTTP",
			opts: []Option{WithPlainHTTP(true)},
		},
		{
			name: "with TLS client config",
			opts: []Option{
				WithTLSClientConfig(pub, priv, ca),
			},
		},
		{
			name: "with insecure skip TLS verify",
			opts: []Option{
				WithInsecureSkipTLSVerify(true),
			},
		},
		{
			name: "with cert and key only",
			opts: []Option{
				WithTLSClientConfig(pub, priv, ""),
			},
		},
		{
			name: "with CA file only",
			opts: []Option{
				WithTLSClientConfig("", "", ca),
			},
		},
		{
			name: "default client without options",
			opts: []Option{},
		},
		{
			name: "invalid cert file",
			opts: []Option{
				WithTLSClientConfig("/non/existent/cert.pem", priv, ca),
			},
			expectError:   true,
			errorContains: "can't create TLS config",
		},
		{
			name: "invalid key file",
			opts: []Option{
				WithTLSClientConfig(pub, "/non/existent/key.pem", ca),
			},
			expectError:   true,
			errorContains: "can't create TLS config",
		},
		{
			name: "invalid CA file",
			opts: []Option{
				WithTLSClientConfig("", "", "/non/existent/ca.crt"),
			},
			expectError:   true,
			errorContains: "can't create TLS config",
		},
		{
			name: "combined TLS options",
			opts: []Option{
				WithTLSClientConfig(pub, priv, ca),
				WithInsecureSkipTLSVerify(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pusher, err := NewOCIPusher(tt.opts...)
			if err != nil {
				t.Fatal(err)
			}

			op, ok := pusher.(*OCIPusher)
			if !ok {
				t.Fatal("Expected *OCIPusher")
			}

			client, err := op.newRegistryClient()
			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if client == nil {
					t.Fatal("Expected non-nil registry client")
				}
			}
		})
	}
}

func TestOCIPusher_Push_ChartOperations(t *testing.T) {
	// Path to test charts
	chartWithProvPath := "../../pkg/cmd/testdata/testcharts/signtest-0.1.0.tgz"

	tests := []struct {
		name          string
		chartRef      string
		href          string
		options       []Option
		setupFunc     func(t *testing.T) (string, func())
		expectError   bool
		errorContains string
	}{
		{
			name:          "invalid chart file",
			chartRef:      "../../pkg/action/testdata/charts/corrupted-compressed-chart.tgz",
			href:          "oci://localhost:5000/test",
			expectError:   true,
			errorContains: "does not appear to be a gzipped archive",
		},
		{
			name:     "push with provenance file - loading phase",
			chartRef: chartWithProvPath,
			href:     "oci://registry.example.com/charts",
			setupFunc: func(t *testing.T) (string, func()) {
				t.Helper()
				// Copy chart and create a .prov file for it
				tempDir := t.TempDir()
				tempChart := filepath.Join(tempDir, "signtest-0.1.0.tgz")
				tempProv := filepath.Join(tempDir, "signtest-0.1.0.tgz.prov")

				// Copy chart file
				src, err := os.Open(chartWithProvPath)
				if err != nil {
					t.Fatal(err)
				}
				defer src.Close()

				dst, err := os.Create(tempChart)
				if err != nil {
					t.Fatal(err)
				}

				if _, err := io.Copy(dst, src); err != nil {
					t.Fatal(err)
				}
				dst.Close()

				// Create provenance file
				if err := os.WriteFile(tempProv, []byte("test provenance data"), 0644); err != nil {
					t.Fatal(err)
				}

				return tempChart, func() {}
			},
			expectError:   true, // Will fail at the registry push step
			errorContains: "",   // Error depends on registry client behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartRef := tt.chartRef
			var cleanup func()

			if tt.setupFunc != nil {
				chartRef, cleanup = tt.setupFunc(t)
				if cleanup != nil {
					defer cleanup()
				}
			}

			// Skip test if chart file doesn't exist and we're not expecting an error
			if _, err := os.Stat(chartRef); err != nil && !tt.expectError {
				t.Skipf("Test chart %s not found, skipping test", chartRef)
			}

			pusher, err := NewOCIPusher(tt.options...)
			if err != nil {
				t.Fatal(err)
			}

			err = pusher.Push(chartRef, tt.href)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestOCIPusher_Push_MultipleOptions(t *testing.T) {
	chartPath := "../../pkg/cmd/testdata/testcharts/compressedchart-0.1.0.tgz"

	// Skip test if chart file doesn't exist
	if _, err := os.Stat(chartPath); err != nil {
		t.Skipf("Test chart %s not found, skipping test", chartPath)
	}

	pusher, err := NewOCIPusher()
	if err != nil {
		t.Fatal(err)
	}

	// Test that multiple options are applied correctly
	err = pusher.Push(chartPath, "oci://localhost:5000/test",
		WithPlainHTTP(true),
		WithInsecureSkipTLSVerify(true),
	)

	// We expect an error since we're not actually pushing to a registry
	if err == nil {
		t.Fatal("Expected error when pushing without a valid registry")
	}

	// Verify options were applied
	op := pusher.(*OCIPusher)
	if !op.opts.plainHTTP {
		t.Error("Expected plainHTTP option to be applied")
	}
	if !op.opts.insecureSkipTLSverify {
		t.Error("Expected insecureSkipTLSverify option to be applied")
	}
}
