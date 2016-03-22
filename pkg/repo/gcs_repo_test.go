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
	"github.com/kubernetes/helm/pkg/chart"

	"os"
	"reflect"
	"regexp"
	"testing"
)

var (
	TestArchiveURL         = os.Getenv("TEST_ARCHIVE_URL")
	TestChartName          = "frobnitz"
	TestChartVersion       = "0.0.1"
	TestArchiveName        = TestChartName + "-" + TestChartVersion + ".tgz"
	TestChartFile          = "testdata/frobnitz/Chart.yaml"
	TestShouldFindRegex    = regexp.MustCompile(TestArchiveName)
	TestShouldNotFindRegex = regexp.MustCompile("foobar")
)

func TestValidGSURL(t *testing.T) {
	tr := getTestRepo(t)
	err := validateRepo(tr, TestRepoName, TestRepoURL, TestRepoCredentialName, TestRepoFormat, TestRepoType)
	if err != nil {
		t.Fatal(err)
	}

	wantBucket := TestRepoBucket
	haveBucket := tr.GetBucket()
	if haveBucket != wantBucket {
		t.Fatalf("unexpected bucket; want: %s, have %s.", wantBucket, haveBucket)
	}
}

func TestInvalidGSURL(t *testing.T) {
	var invalidGSURL = "https://valid.url/wrong/scheme"
	_, err := NewGCSRepo(TestRepoName, invalidGSURL, TestRepoCredentialName, nil)
	if err == nil {
		t.Fatalf("expected error did not occur for invalid GS URL")
	}
}

func TestListCharts(t *testing.T) {
	tr := getTestRepo(t)
	charts, err := tr.ListCharts(nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(charts) != 1 {
		t.Fatalf("expected one chart in list, got %d", len(charts))
	}

	haveName := charts[0]
	wantName := TestArchiveName
	if haveName != wantName {
		t.Fatalf("expected chart named %s, got %s", wantName, haveName)
	}
}

func TestListChartsWithShouldFindRegex(t *testing.T) {
	tr := getTestRepo(t)
	charts, err := tr.ListCharts(TestShouldFindRegex)
	if err != nil {
		t.Fatal(err)
	}

	if len(charts) != 1 {
		t.Fatalf("expected one chart to match regex, got %d", len(charts))
	}
}

func TestListChartsWithShouldNotFindRegex(t *testing.T) {
	tr := getTestRepo(t)
	charts, err := tr.ListCharts(TestShouldNotFindRegex)
	if err != nil {
		t.Fatal(err)
	}

	if len(charts) != 0 {
		t.Fatalf("expected zero charts to match regex, got %d", len(charts))
	}
}

func TestGetChart(t *testing.T) {
	tr := getTestRepo(t)
	tc, err := tr.GetChart(TestArchiveName)
	if err != nil {
		t.Fatal(err)
	}

	haveFile := tc.Chartfile()
	wantFile, err := chart.LoadChartfile(TestChartFile)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(wantFile, haveFile) {
		t.Fatalf("retrieved invalid chart\nwant:%#v\nhave:\n%#v\n", wantFile, haveFile)
	}
}

func TestGetChartWithInvalidName(t *testing.T) {
	tr := getTestRepo(t)
	_, err := tr.GetChart("NotAValidArchiveName")
	if err == nil {
		t.Fatalf("found chart using invalid archive name")
	}
}

func getTestRepo(t *testing.T) IStorageRepo {
	tr, err := NewGCSRepo(TestRepoName, TestRepoURL, TestRepoCredentialName, nil)
	if err != nil {
		t.Fatal(err)
	}

	return tr
}
