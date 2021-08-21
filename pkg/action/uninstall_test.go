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

func uninstallAction(t *testing.T) *Uninstall {
	config := actionConfigFixture(t)
	unAction := NewUninstall(config)
	return unAction
}

func TestUninstallRelease_deleteRelease(t *testing.T) {
	is := assert.New(t)

	unAction := uninstallAction(t)
	unAction.DisableHooks = true
	unAction.DryRun = false
	unAction.KeepHistory = true

	rel := releaseStub()
	rel.Name = "keep-secret"
	rel.Manifest = `{
		"apiVersion": "v1",
		"kind": "Secret",
		"metadata": {
		  "name": "secret",
		  "annotations": {
			"helm.sh/resource-policy": "keep"
		  }
		},
		"type": "Opaque",
		"data": {
		  "password": "password"
		}
	}`
	unAction.cfg.Releases.Create(rel)
	res, err := unAction.Run(rel.Name)
	is.NoError(err)
	expected := `These resources were kept due to the resource policy:
[Secret] secret
`
	is.Contains(res.Info, expected)
}
