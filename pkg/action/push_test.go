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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPushWithPushConfig(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewPushWithOpts(WithPushConfig(config))

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestNewPushWithTLSClientConfig(t *testing.T) {
	certFile := "certFile"
	keyFile := "keyFile"
	caFile := "caFile"
	client := NewPushWithOpts(WithTLSClientConfig(certFile, keyFile, caFile))

	assert.NotNil(t, client)
	assert.Equal(t, certFile, client.certFile)
	assert.Equal(t, keyFile, client.keyFile)
	assert.Equal(t, caFile, client.caFile)
}

func TestNewPushWithInsecureSkipTLSVerify(t *testing.T) {
	client := NewPushWithOpts(WithInsecureSkipTLSVerify(true))

	assert.NotNil(t, client)
	assert.Equal(t, true, client.insecureSkipTLSVerify)
}

func TestNewPushWithPlainHTTP(t *testing.T) {
	client := NewPushWithOpts(WithPlainHTTP(true))

	assert.NotNil(t, client)
	assert.Equal(t, true, client.plainHTTP)
}

func TestNewPushWithPushOptWriter(t *testing.T) {
	buf := new(bytes.Buffer)
	client := NewPushWithOpts(WithPushOptWriter(buf))

	assert.NotNil(t, client)
	assert.Equal(t, buf, client.out)
}
