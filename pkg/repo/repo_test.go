/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package repo

import (
	"testing"
)

func TestValidURL(t *testing.T) {
	var wantName = "wantName"
	var wantType = "wantType"
	var validURL = "http://valid/url"
	var wantFormat = "wantFormat"

	tr, err := newRepo(wantName, validURL, wantFormat, wantType)
	if err != nil {
		t.Fatal(err)
	}

	haveName := tr.GetRepoName()
	if haveName != wantName {
		t.Fatalf("unexpected repo name; want: %s, have %s.", wantName, haveName)
	}

	haveType := string(tr.GetRepoType())
	if haveType != wantType {
		t.Fatalf("unexpected repo type; want: %s, have %s.", wantType, haveType)
	}

	haveURL := tr.GetRepoURL()
	if haveURL != validURL {
		t.Fatalf("unexpected repo url; want: %s, have %s.", validURL, haveURL)
	}

	haveFormat := string(tr.GetRepoFormat())
	if haveFormat != wantFormat {
		t.Fatalf("unexpected repo format; want: %s, have %s.", wantFormat, haveFormat)
	}
}

func TestInvalidURL(t *testing.T) {
	_, err := newRepo("testName", "%:invalid&url:%", "testFormat", "testType")
	if err == nil {
		t.Fatalf("expected error did not occur for invalid URL")
	}
}
