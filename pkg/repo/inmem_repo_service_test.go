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
	"reflect"
	"testing"
)

func TestService(t *testing.T) {
	rs := NewInmemRepoService()
	repos, err := rs.List()
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 1 {
		t.Fatalf("unexpected repo count; want: %d, have %d.", 1, len(repos))
	}

	tr := repos[0]
	if err := validateRepo(tr, GCSPublicRepoName, GCSPublicRepoURL, "", GCSRepoFormat, GCSRepoType); err != nil {
		t.Fatal(err)
	}

	r1, err := rs.Get(GCSPublicRepoName)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(r1, tr) {
		t.Fatalf("invalid repo returned; want: %#v, have %#v.", tr, r1)
	}

	r2, err := rs.GetByURL(GCSPublicRepoURL)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(r2, tr) {
		t.Fatalf("invalid repo returned; want: %#v, have %#v.", tr, r2)
	}

	if err := rs.Delete(GCSPublicRepoName); err != nil {
		t.Fatal(err)
	}

	if _, err := rs.Get(GCSPublicRepoName); err == nil {
		t.Fatalf("deleted repo named %s returned", GCSPublicRepoName)
	}
}

func TestGetRepoWithInvalidName(t *testing.T) {
	invalidName := "InvalidRepoName"
	rs := NewInmemRepoService()
	_, err := rs.Get(invalidName)
	if err == nil {
		t.Fatalf("found repo with invalid name: %s", invalidName)
	}
}

func TestGetRepoWithInvalidURL(t *testing.T) {
	invalidURL := "https://not.a.valid/url"
	rs := NewInmemRepoService()
	_, err := rs.GetByURL(invalidURL)
	if err == nil {
		t.Fatalf("found repo with invalid URL: %s", invalidURL)
	}
}

func TestDeleteRepoWithInvalidName(t *testing.T) {
	invalidName := "InvalidRepoName"
	rs := NewInmemRepoService()
	err := rs.Delete(invalidName)
	if err == nil {
		t.Fatalf("deleted repo with invalid name: %s", invalidName)
	}
}
