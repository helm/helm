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
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	kubefake "helm.sh/helm/v4/pkg/kube/fake"
)

func TestNewRollback(t *testing.T) {
	config := actionConfigFixture(t)
	client := NewRollback(config)

	assert.NotNil(t, client)
	assert.Equal(t, config, client.cfg)
}

func TestRollbackRun_UnreachableKubeClient(t *testing.T) {
	config := actionConfigFixture(t)
	failingKubeClient := kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}, DummyResources: nil}
	failingKubeClient.ConnectionError = errors.New("connection refused")
	config.KubeClient = &failingKubeClient

	client := NewRollback(config)
	assert.Error(t, client.Run(""))
}
