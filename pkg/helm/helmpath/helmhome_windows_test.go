// Copyright 2016 The Kubernetes Authors All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

// http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build windows

package helmpath

import (
	"testing"
)

func TestHelmHome(t *testing.T) {
	hh := Home("r:\\")
	isEq := func(t *testing.T, a, b string) {
		if a != b {
			t.Errorf("Expected %q, got %q", b, a)
		}
	}

	isEq(t, hh.String(), "r:\\")
	isEq(t, hh.Repository(), "r:\\repository")
	isEq(t, hh.RepositoryFile(), "r:\\repository\\repositories.yaml")
	isEq(t, hh.LocalRepository(), "r:\\repository\\local")
	isEq(t, hh.Cache(), "r:\\repository\\cache")
	isEq(t, hh.CacheIndex("t"), "r:\\repository\\cache\\t-index.yaml")
	isEq(t, hh.Starters(), "r:\\starters")
	isEq(t, hh.Archive(), "r:\\cache\\archive")
}
