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
	"testing"
)

var filename = "./testdata/test_credentials_file.yaml"

type filebasedTestCase struct {
	name   string
	exp    *Credential
	expErr error
}

func TestNotExistFilebased(t *testing.T) {
	cp := getProvider(t)
	tc := &testCase{"nonexistent", nil, createMissingError("nonexistent")}
	testGetCredential(t, cp, tc)
}

func TestGetApiTokenFilebased(t *testing.T) {
	cp := getProvider(t)
	tc := &testCase{"test1", &Credential{APIToken: "token"}, nil}
	testGetCredential(t, cp, tc)
}

func TestSetAndGetBasicAuthFilebased(t *testing.T) {
	cp := getProvider(t)
	ba := BasicAuthCredential{Username: "user", Password: "password"}
	tc := &testCase{"test2", &Credential{BasicAuth: ba}, nil}
	testGetCredential(t, cp, tc)
}

func getProvider(t *testing.T) ICredentialProvider {
	cp, err := NewFilebasedCredentialProvider(filename)
	if err != nil {
		t.Fatalf("cannot create a new provider from file %s: %s", filename, err)
	}

	return cp
}
