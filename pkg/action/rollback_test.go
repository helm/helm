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

func rollbackAction(t *testing.T) *Rollback {
	config := actionConfigFixture(t)
	rollbackAction := NewRollback(config)

	return rollbackAction
}

func TestRollbackToReleaseWithExternalFile(t *testing.T) {
	is := assert.New(t)
	vals := map[string]interface{}{}

	chartVersion1 := buildChart(withExternalFileTemplate(ExternalFileRelPath))
	chartVersion2 := buildChart()

	instAction := installAction(t)
	instAction.ExternalPaths = append(instAction.ExternalPaths, ExternalFileRelPath)
	relVersion1, err := instAction.Run(chartVersion1, vals)
	is.Contains(relVersion1.Manifest, "out-of-chart-dir")
	is.NoError(err)

	upAction := upgradeAction(t)
	err = upAction.cfg.Releases.Create(relVersion1)
	is.NoError(err)
	relVersion2, err := upAction.Run(relVersion1.Name, chartVersion2, vals)
	is.NotContains(relVersion2.Manifest, "out-out-chart-dir")
	is.NoError(err)

	rollAction := rollbackAction(t)
	err = rollAction.cfg.Releases.Create(relVersion1)
	is.NoError(err)
	err = rollAction.cfg.Releases.Create(relVersion2)
	is.NoError(err)
	currentRelease, targetRelease, err := rollAction.prepareRollback(relVersion2.Name)
	is.NoError(err)
	relVersion3, err := rollAction.performRollback(currentRelease, targetRelease)
	is.NoError(err)

	is.Contains(relVersion3.Manifest, "out-of-chart-dir")
}
