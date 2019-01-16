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

package helmpath

import (
	"os"
	"runtime"
	"testing"
)

func StringEquals(t *testing.T, a, b string) {
	if a != b {
		t.Error(runtime.GOOS)
		t.Errorf("Expected %q, got %q", a, b)
	}
}

func returns(what bool) func() bool { return func() bool { return what } }

func TestGetDefaultConfigHome(t *testing.T) {
	var _OldDefaultHelmHomeExists = OldDefaultHelmHomeExists
	var _DefaultHelmHomeExists = DefaultHelmHomeExists

	OldDefaultHelmHomeExists = returns(false)
	DefaultHelmHomeExists = returns(false)
	StringEquals(t, GetDefaultConfigHome(os.Stdout), defaultHelmHome)

	OldDefaultHelmHomeExists = returns(true)
	DefaultHelmHomeExists = returns(false)
	StringEquals(t, GetDefaultConfigHome(os.Stdout), oldDefaultHelmHome)

	OldDefaultHelmHomeExists = returns(false)
	DefaultHelmHomeExists = returns(true)
	StringEquals(t, GetDefaultConfigHome(os.Stdout), defaultHelmHome)

	OldDefaultHelmHomeExists = returns(true)
	DefaultHelmHomeExists = returns(true)
	StringEquals(t, GetDefaultConfigHome(os.Stdout), defaultHelmHome)

	OldDefaultHelmHomeExists = _OldDefaultHelmHomeExists
	DefaultHelmHomeExists = _DefaultHelmHomeExists
}
