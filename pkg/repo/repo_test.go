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

var (
	TestRepoBucket         = "kubernetes-charts-testing"
	TestRepoURL            = "gs://" + TestRepoBucket
	TestRepoType           = GCSRepoType
	TestRepoFormat         = GCSRepoFormat
	TestRepoCredentialName = "default"
)

func TestValidRepoURL(t *testing.T) {
	tr, err := NewRepo(TestRepoURL, TestRepoCredentialName, string(TestRepoFormat), string(TestRepoType))
	if err != nil {
		t.Fatal(err)
	}

	if err := validateRepo(tr, TestRepoURL, TestRepoCredentialName, TestRepoFormat, TestRepoType); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidRepoURL(t *testing.T) {
	_, err := newRepo("%:invalid&url:%", TestRepoCredentialName, TestRepoFormat, TestRepoType)
	if err == nil {
		t.Fatalf("expected error did not occur for invalid URL")
	}
}

func TestDefaultCredentialName(t *testing.T) {
	tr, err := newRepo(TestRepoURL, "", TestRepoFormat, TestRepoType)
	if err != nil {
		t.Fatalf("cannot create repo using default credential name")
	}

	TestRepoCredentialName := "default"
	haveCredentialName := tr.GetCredentialName()
	if haveCredentialName != TestRepoCredentialName {
		t.Fatalf("unexpected credential name; want: %s, have %s.", TestRepoCredentialName, haveCredentialName)
	}
}

func TestInvalidRepoFormat(t *testing.T) {
	_, err := newRepo(TestRepoURL, TestRepoCredentialName, "", TestRepoType)
	if err == nil {
		t.Fatalf("expected error did not occur for invalid format")
	}
}
