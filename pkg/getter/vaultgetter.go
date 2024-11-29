/*
Copyright (c) 2024 Rakuten Symphony India.

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

package getter

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/vault/api"
	"gopkg.in/yaml.v3"
)

// VaultGetter is the struct that handles retrieving data from Vault.
type VaultGetter struct {
	opts   options     // Options for Vault (address, token, etc.)
	client *api.Client // The Vault client
	once   sync.Once   // Ensure the Vault client is initialized only once
}

// Get performs a Get from repo.Getter and returns the body.
func (v *VaultGetter) Get(href string, options ...Option) (*bytes.Buffer, error) {
	for _, opt := range options {
		opt(&v.opts)
	}
	return v.get(href)
}

func (v *VaultGetter) get(href string) (*bytes.Buffer, error) {
	// Initialize the Vault client
	client, err := v.vaultClient()
	if err != nil {
		return nil, err
	}

	// Fetch the values from Vault using the Vault client
	valPath := strings.TrimPrefix(href, "vault://")
	val, err := client.Logical().Read(valPath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch values from Vault: %v", err)
	}

	// Ensure the values contains data
	if val == nil || val.Data == nil {
		return nil, fmt.Errorf("no data found at Vault path: %s", valPath)
	}

	// Retrieve the data (assumed to be a string in the Vault response)
	data, ok := val.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format at Vault path: %s", href)
	}

	// Unwrap the "values" key if it exists
	if values, ok := data["values"].(string); ok {
		data = make(map[string]interface{})
		if err := yaml.Unmarshal([]byte(values), &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal values: %v", err)
		}
	}

	// Check if the data is in properties format
	if properties, ok := data["properties"].(string); ok {
		data = make(map[string]interface{})
		for _, line := range strings.Split(properties, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid properties line: %s", line)
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			data[key] = value
		}
	}

	// Return the data in a byte buffer
	buf := bytes.NewBuffer(nil)
	if err = yaml.NewEncoder(buf).Encode(data); err != nil {
		return nil, fmt.Errorf("failed to encode data to YAML: %v", err)
	}

	return buf, nil
}

// NewVaultGetter creates a new instance of VaultGetter.
func NewVaultGetter(options ...Option) (Getter, error) {
	var v VaultGetter

	for _, opt := range options {
		opt(&v.opts)
	}

	return &v, nil
}

func (v *VaultGetter) vaultClient() (*api.Client, error) {
	if v.client != nil {
		return v.client, nil
	}

	var config *api.Config

	// Use sync.Once to initialize the Vault client only once
	v.once.Do(func() {
		config = &api.Config{
			Address: v.opts.address, // Vault URL is set from options
		}
	})

	// Configure TLS if needed
	if v.opts.caFile != "" || v.opts.insecureSkipVerifyTLS {
		tlsConfig := &api.TLSConfig{
			CACert:     v.opts.caFile,
			Insecure:   v.opts.insecureSkipVerifyTLS,
			ClientCert: v.opts.certFile,
			ClientKey:  v.opts.keyFile,
		}
		err := config.ConfigureTLS(tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to configure TLS for Vault client: %v", err)
		}
	}

	// Initialize the Vault client
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %v", err)
	}

	// Set the token for authentication
	client.SetToken(v.opts.token)
	v.client = client

	return v.client, nil
}
