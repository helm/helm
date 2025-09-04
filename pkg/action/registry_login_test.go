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

package action

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRegistryLogin(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRegistryLogin(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestWithCertFile(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRegistryLogin(config)

	certFile := "testdata/cert.pem"
	opt := WithCertFile(certFile)

	assert.Nil(t, opt(client))
	assert.Equal(t, certFile, client.certFile)
}

func TestWithInsecure(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRegistryLogin(config)

	opt := WithInsecure(true)

	assert.Nil(t, opt(client))
	assert.Equal(t, true, client.insecure)
}

func TestWithKeyFile(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRegistryLogin(config)

	keyFile := "testdata/key.pem"
	opt := WithKeyFile(keyFile)

	assert.Nil(t, opt(client))
	assert.Equal(t, keyFile, client.keyFile)
}

func TestWithCAFile(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRegistryLogin(config)

	caFile := "testdata/ca.pem"
	opt := WithCAFile(caFile)

	assert.Nil(t, opt(client))
	assert.Equal(t, caFile, client.caFile)
}

func TestWithPlainHTTPLogin(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRegistryLogin(config)

	opt := WithPlainHTTPLogin(true)

	assert.Nil(t, opt(client))
	assert.Equal(t, true, client.plainHTTP)
}
