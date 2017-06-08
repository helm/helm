/*
Copyright 2017 The Kubernetes Authors All rights reserved.
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
	"os"
	"testing"
)

func TestProvider(t *testing.T) {
	p := Provider{
		[]string{"one", "three"},
		func(h, e, l, m string) (Getter, error) { return nil, nil },
	}

	if !p.Provides("three") {
		t.Error("Expected provider to provide three")
	}
}

func TestProviders(t *testing.T) {
	ps := Providers{
		{[]string{"one", "three"}, func(h, e, l, m string) (Getter, error) { return nil, nil }},
		{[]string{"two", "four"}, func(h, e, l, m string) (Getter, error) { return nil, nil }},
	}

	if _, err := ps.ByScheme("one"); err != nil {
		t.Error(err)
	}
	if _, err := ps.ByScheme("four"); err != nil {
		t.Error(err)
	}

	if _, err := ps.ByScheme("five"); err == nil {
		t.Error("Did not expect handler for five")
	}
}

func TestAll(t *testing.T) {
	oldhh := os.Getenv("HELM_HOME")
	defer os.Setenv("HELM_HOME", oldhh)
	os.Setenv("HELM_HOME", "")

	env := hh(false)

	all := All(env)
	if len(all) != 3 {
		t.Errorf("expected 3 providers (default plus two plugins), got %d", len(all))
	}

	if _, err := all.ByScheme("test2"); err != nil {
		t.Error(err)
	}
}

func TestByScheme(t *testing.T) {
	oldhh := os.Getenv("HELM_HOME")
	defer os.Setenv("HELM_HOME", oldhh)
	os.Setenv("HELM_HOME", "")

	env := hh(false)
	if _, err := ByScheme("test", env); err != nil {
		t.Error(err)
	}
	if _, err := ByScheme("https", env); err != nil {
		t.Error(err)
	}
}
