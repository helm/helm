/*
Copyright The Helm maintainers

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
package notify

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	"k8s.io/helm/pkg/version"
)

type URLHandler struct {
	releases Releases
}

func (h *URLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(h.releases)
	if err != nil {
		fmt.Println(err)
		return
	}
	w.Header().Set("Content-Type", "application/javascript")
	fmt.Fprintf(w, string(b))
}

func TestWorkingIfTime(t *testing.T) {

	curSemVer, err := semver.NewVersion(version.GetVersion())
	if err != nil {
		t.Errorf("internal version (%s) could not be parsed: %s", version.GetVersion(), err)
	}

	nextVer := curSemVer.IncMinor()
	nextVerString := "v" + nextVer.String()

	tests := []struct {
		checked    bool
		version    string
		newVersion string
		duration   int64
	}{
		{true, nextVerString, nextVer.String(), 10},
		{true, "v" + curSemVer.String(), "", 10},
		{false, nextVerString, "", 10000},
	}

	tmpTime := time.Now()
	tmpTime = tmpTime.Add(-100 * time.Second)

	for i, tc := range tests {
		handler := &URLHandler{
			releases: []Release{{Version: tc.version}},
		}
		server := httptest.NewServer(handler)

		checked, newVer, err := IfTime(tmpTime, time.Duration(tc.duration)*time.Second, server.URL)
		if err != nil {
			t.Errorf("unexpected error checking IfTime: %s", err)
		}
		if checked != tc.checked {
			t.Errorf("new version check of %t but got %t for test case %d", tc.checked, checked, i)
		}

		if tc.newVersion != newVer {
			t.Errorf("expected a new version of %qbut got %q for test case %d", tc.newVersion, newVer, i)
		}
	}
}

func TestWorkingIfTimeFromFile(t *testing.T) {

	curSemVer, err := semver.NewVersion(version.GetVersion())
	if err != nil {
		t.Errorf("internal version (%s) could not be parsed: %s", version.GetVersion(), err)
	}

	nextVer := curSemVer.IncMinor()

	tests := []struct {
		version    *semver.Version
		newVersion string
		duration   int64
	}{
		{&nextVer, nextVer.String(), 10},
		{curSemVer, "", 10},
		{&nextVer, "", 10000},
	}

	tempDir, err := ioutil.TempDir("", "helm-update_check-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	lastUpdatePath := filepath.Join(tempDir, "last_update_check")

	for i, tc := range tests {

		tmpTime := time.Now()
		tmpTime = tmpTime.Add(-100 * time.Second)
		ioutil.WriteFile(lastUpdatePath, []byte(tmpTime.Format(timeLayout)), 0644)

		handler := &URLHandler{
			releases: []Release{{Version: "v" + tc.version.String()}},
		}
		server := httptest.NewServer(handler)

		newVer, err := IfTimeFromFile(lastUpdatePath, tc.duration, server.URL)
		if err != nil {
			t.Errorf("unexpected error checking IfTime: %s", err)
		}

		if tc.newVersion != newVer {
			t.Errorf("expected a new version of %qbut got %q for test case %d", tc.newVersion, newVer, i)
		}
	}
}

type URLHandlerNone struct{}

func (h *URLHandlerNone) ServeHTTP(w http.ResponseWriter, r *http.Request) {
}

func TestBadURLIfTime(t *testing.T) {
	handler := &URLHandlerNone{}
	server := httptest.NewServer(handler)

	tmpTime := time.Now()
	tmpTime = tmpTime.Add(-100 * time.Second)
	_, _, err := IfTime(tmpTime, time.Duration(10)*time.Second, server.URL)

	if err == nil {
		t.Error("expected error with handler but did not get one")
	}
}

func TestBadURLIfTimeFromFile(t *testing.T) {
	handler := &URLHandlerNone{}
	server := httptest.NewServer(handler)

	tmpTime := time.Now()
	tmpTime = tmpTime.Add(-100 * time.Second)
	tempDir, err := ioutil.TempDir("", "helm-update_check-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	lastUpdatePath := filepath.Join(tempDir, "last_update_check")
	ioutil.WriteFile(lastUpdatePath, []byte(tmpTime.Format(timeLayout)), 0644)

	_, err = IfTimeFromFile(lastUpdatePath, 10, server.URL)

	if err == nil {
		t.Error("expected error with handler but did not get one")
	}
}

func TestNoFileIfTimeFromFile(t *testing.T) {
	handler := &URLHandler{
		releases: []Release{{Version: "v1.2.3"}},
	}
	server := httptest.NewServer(handler)

	tempDir, err := ioutil.TempDir("", "helm-update_check-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	lastUpdatePath := filepath.Join(tempDir, "last_update_check")

	newVer, err := IfTimeFromFile(lastUpdatePath, 10, server.URL)

	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if newVer != "" {
		t.Errorf("expected an empty string version but got %s", newVer)
	}

	content, err := ioutil.ReadFile(lastUpdatePath)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	_, err = time.Parse(timeLayout, string(content))
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}
