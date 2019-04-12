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

package repotest

import (
	"strings"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer()

	if err := s.Shutdown(); err != nil {
		t.Error(err)
	}

	// restart to test Close
	s.Start()

	if err := s.Close(); err != nil {
		t.Error(err)
	}
}

func TestNewServerStart(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("did not recover when calling Start on a Server that already started")
		}
	}()

	NewServer().Start()
}

func TestUnstartedServerDoubleStart(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("did not recover when calling Start on a Server that already started")
		}
	}()

	s := NewUnstartedServer()
	s.Start()
	s.Start()
}

func TestURL(t *testing.T) {
	s := NewUnstartedServer()

	expected := ""
	actual := s.URL()
	if expected != actual {
		t.Errorf("expected '%s, got '%s'", expected, actual)
	}

	s.Start()

	if !strings.HasPrefix(s.URL(), "localhost:") {
		t.Errorf("expected url to start with 'localhost:', got '%s'", s.URL())
	}

	s.Close()

	// test that we ignore this parameter
	s.Config.HTTP.Net = "unix"

	s.Start()
	defer s.Close()

	if !strings.HasPrefix(s.URL(), "localhost:") {
		t.Errorf("expected url to start with 'localhost:', got '%s'", s.URL())
	}
}
