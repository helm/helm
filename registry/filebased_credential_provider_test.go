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

package registry

import (
	"testing"

	"github.com/kubernetes/deployment-manager/common"
)

var filename = "./test/test_credentials_file.yaml"

type filebasedTestCase struct {
	name   string
	exp    *common.RegistryCredential
	expErr error
}

func TestNotExistFilebased(t *testing.T) {
	cp, err := NewFilebasedCredentialProvider(filename)
	if err != nil {
		t.Fatalf("Failed to create a new FilebasedCredentialProvider %s : %v", filename, err)
	}
	tc := &testCase{"nonexistent", nil, createMissingError("nonexistent")}
	testGetCredential(t, cp, tc)
}

func TestGetApiTokenFilebased(t *testing.T) {
	cp, err := NewFilebasedCredentialProvider(filename)
	if err != nil {
		t.Fatalf("Failed to create a new FilebasedCredentialProvider %s : %v", filename, err)
	}
	tc := &testCase{"test1", &common.RegistryCredential{APIToken: "token"}, nil}
	testGetCredential(t, cp, tc)
}

func TestSetAndGetBasicAuthFilebased(t *testing.T) {
	cp, err := NewFilebasedCredentialProvider(filename)
	if err != nil {
		t.Fatalf("Failed to create a new FilebasedCredentialProvider %s : %v", filename, err)
	}
	tc := &testCase{"test2",
		&common.RegistryCredential{
			BasicAuth: common.BasicAuthCredential{Username: "user", Password: "password"}}, nil}
	testGetCredential(t, cp, tc)
}
