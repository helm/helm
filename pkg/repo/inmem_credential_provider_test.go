/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"fmt"
	"reflect"
	"testing"
)

type testCase struct {
	name   string
	exp    *RepoCredential
	expErr error
}

func createMissingError(name string) error {
	return fmt.Errorf("no such credential: %s", name)
}

func testGetCredential(t *testing.T, cp CredentialProvider, tc *testCase) {
	actual, actualErr := cp.GetCredential(tc.name)
	if !reflect.DeepEqual(actual, tc.exp) {
		t.Fatalf("test case %s failed: want: %#v, have: %#v", tc.name, tc.exp, actual)
	}

	if !reflect.DeepEqual(actualErr, tc.expErr) {
		t.Fatalf("test case %s failed: want: %s, have: %s", tc.name, tc.expErr, actualErr)
	}
}

func verifySetAndGetCredential(t *testing.T, cp CredentialProvider, tc *testCase) {
	err := cp.SetCredential(tc.name, tc.exp)
	if err != nil {
		t.Fatalf("test case %s failed: cannot set credential: %v", tc.name, err)
	}

	testGetCredential(t, cp, tc)
}

func TestNotExist(t *testing.T) {
	cp := NewInmemCredentialProvider()
	tc := &testCase{"nonexistent", nil, createMissingError("nonexistent")}
	testGetCredential(t, cp, tc)
}

func TestSetAndGetApiToken(t *testing.T) {
	cp := NewInmemCredentialProvider()
	tc := &testCase{"testcredential", &RepoCredential{APIToken: "some token here"}, nil}
	verifySetAndGetCredential(t, cp, tc)
}

func TestSetAndGetBasicAuth(t *testing.T) {
	cp := NewInmemCredentialProvider()
	ba := BasicAuthCredential{Username: "user", Password: "pass"}
	tc := &testCase{"testcredential", &RepoCredential{BasicAuth: ba}, nil}
	verifySetAndGetCredential(t, cp, tc)
}
