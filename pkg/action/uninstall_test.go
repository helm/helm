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

func TestUninstall_deleteRelease(t *testing.T) {
	is := assert.New(t)
	rel := releaseStub()
	rel.Chart = buildChart(withKeepAnnoManifestTemplate())
	rel.Manifest = manifestWithKeepAnno
	config := actionConfigFixture(t)
	unisAction := NewUninstall(config)
	str, errs := unisAction.deleteRelease(rel)
	is.Len(errs, 0)
	is.Equal("Pod/pod-keep\n", str)
}
