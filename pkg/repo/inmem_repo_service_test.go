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
	repos, err := rs.ListRepos()
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 1 {
		t.Fatalf("unexpected repo count; want: %d, have %d.", 1, len(repos))
	}

	tr, err := rs.GetRepoByURL(repos[0])
	if err != nil {
		t.Fatal(err)
	}

	if err := validateRepo(tr, GCSPublicRepoURL, "", GCSRepoFormat, GCSRepoType); err != nil {
		t.Fatal(err)
	}

	r1, err := rs.GetRepoByURL(GCSPublicRepoURL)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(r1, tr) {
		t.Fatalf("invalid repo returned; want: %#v, have %#v.", tr, r1)
	}

	URL := GCSPublicRepoURL + TestArchiveName
	r2, err := rs.GetRepoByChartURL(URL)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(r2, tr) {
		t.Fatalf("invalid repo returned; want: %#v, have %#v.", tr, r2)
	}

	if err := rs.DeleteRepo(GCSPublicRepoURL); err != nil {
		t.Fatal(err)
	}

	if _, err := rs.GetRepoByURL(GCSPublicRepoURL); err == nil {
		t.Fatalf("deleted repo with URL %s returned", GCSPublicRepoURL)
	}
}

func TestCreateRepoWithDuplicateURL(t *testing.T) {
	rs := NewInmemRepoService()
	r, err := newRepo(GCSPublicRepoURL, "", GCSRepoFormat, GCSRepoType)
	if err != nil {
		t.Fatalf("cannot create test repo: %s", err)
	}

	if err := rs.CreateRepo(r); err == nil {
		t.Fatalf("created repo with duplicate URL: %s", GCSPublicRepoURL)
	}
}

func TestGetRepoWithInvalidURL(t *testing.T) {
	invalidURL := "https://not.a.valid/url"
	rs := NewInmemRepoService()
	_, err := rs.GetRepoByURL(invalidURL)
	if err == nil {
		t.Fatalf("found repo with invalid URL: %s", invalidURL)
	}
}

func TestGetRepoWithInvalidChartURL(t *testing.T) {
	invalidURL := "https://not.a.valid/url"
	rs := NewInmemRepoService()
	_, err := rs.GetRepoByChartURL(invalidURL)
	if err == nil {
		t.Fatalf("found repo with invalid chart URL: %s", invalidURL)
	}
}

func TestDeleteRepoWithInvalidURL(t *testing.T) {
	invalidURL := "https://not.a.valid/url"
	rs := NewInmemRepoService()
	err := rs.DeleteRepo(invalidURL)
	if err == nil {
		t.Fatalf("deleted repo with invalid name: %s", invalidURL)
	}
}
