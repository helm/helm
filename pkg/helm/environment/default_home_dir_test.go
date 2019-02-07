// Copyright The Helm Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package environment

import (
	"runtime"
	"testing"
)

func StringEquals(t *testing.T, a, b string) {
	if a != b {
		t.Error(runtime.GOOS)
		t.Errorf("Expected %q, got %q", a, b)
	}
}

type WithNewHome struct{ DefaultConfigHomePath }

func (WithNewHome) xdgHomeExists() bool   { return true }
func (WithNewHome) basicHomeExists() bool { return false }

type WithOldHome struct{ DefaultConfigHomePath }

func (WithOldHome) xdgHomeExists() bool   { return false }
func (WithOldHome) basicHomeExists() bool { return true }

type WithNoHome struct{ DefaultConfigHomePath }

func (WithNoHome) xdgHomeExists() bool   { return false }
func (WithNoHome) basicHomeExists() bool { return false }

type WithAllHomes struct{ DefaultConfigHomePath }

func (WithAllHomes) xdgHomeExists() bool   { return true }
func (WithAllHomes) basicHomeExists() bool { return true }

func TestGetDefaultConfigHome(t *testing.T) {
	oldConfig := ConfigPath

	ConfigPath = WithNewHome{}
	StringEquals(t, GetDefaultConfigHome(), defaultHelmHome)

	ConfigPath = WithOldHome{}
	StringEquals(t, GetDefaultConfigHome(), oldDefaultHelmHome)

	ConfigPath = WithNoHome{}
	StringEquals(t, GetDefaultConfigHome(), defaultHelmHome)

	ConfigPath = WithAllHomes{}
	StringEquals(t, GetDefaultConfigHome(), defaultHelmHome)

	ConfigPath = oldConfig
}
