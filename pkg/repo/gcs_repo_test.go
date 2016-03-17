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
	"github.com/kubernetes/helm/pkg/common"

	"testing"
)

func TestValidGSURL(t *testing.T) {
	var validURL = "gs://bucket"
	tr, err := NewGCSRepo("testName", validURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	wantType := common.GCSRepoType
	haveType := tr.GetRepoType()
	if haveType != wantType {
		t.Fatalf("unexpected repo type; want: %s, have %s.", wantType, haveType)
	}

	wantFormat := GCSRepoFormat
	haveFormat := tr.GetRepoFormat()
	if haveFormat != wantFormat {
		t.Fatalf("unexpected repo format; want: %s, have %s.", wantFormat, haveFormat)
	}

}

func TestInvalidGSURL(t *testing.T) {
	var invalidURL = "https://bucket"
	_, err := NewGCSRepo("testName", invalidURL, nil)
	if err == nil {
		t.Fatalf("expected error did not occur for invalid URL")
	}
}
