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
	"github.com/kubernetes/helm/pkg/common"

	"os"
	"reflect"
	"regexp"
	"testing"
)

var (
	TestArchiveBucket      = os.Getenv("TEST_ARCHIVE_BUCKET")
	TestArchiveName        = "frobnitz-0.0.1.tgz"
	TestChartFile          = "testdata/frobnitz/Chart.yaml"
	TestShouldFindRegex    = regexp.MustCompile(TestArchiveName)
	TestShouldNotFindRegex = regexp.MustCompile("foobar")
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

func TestListCharts(t *testing.T) {
	if TestArchiveBucket != "" {
		tr, err := NewGCSRepo("testName", TestArchiveBucket, nil)
		if err != nil {
			t.Fatal(err)
		}

		charts, err := tr.ListCharts(nil)
		if err != nil {
			t.Fatal(err)
		}

		if len(charts) != 1 {
			t.Fatalf("expected one chart in test bucket, got %d", len(charts))
		}

		name := charts[0]
		if name != TestArchiveName {
			t.Fatalf("expected chart named %s in test bucket, got %s", TestArchiveName, name)
		}
	}
}

func TestListChartsWithShouldFindRegex(t *testing.T) {
	if TestArchiveBucket != "" {
		tr, err := NewGCSRepo("testName", TestArchiveBucket, nil)
		if err != nil {
			t.Fatal(err)
		}

		charts, err := tr.ListCharts(TestShouldFindRegex)
		if err != nil {
			t.Fatal(err)
		}

		if len(charts) != 1 {
			t.Fatalf("expected one chart to match regex, got %d", len(charts))
		}
	}
}

func TestListChartsWithShouldNotFindRegex(t *testing.T) {
	if TestArchiveBucket != "" {
		tr, err := NewGCSRepo("testName", TestArchiveBucket, nil)
		if err != nil {
			t.Fatal(err)
		}

		charts, err := tr.ListCharts(TestShouldNotFindRegex)
		if err != nil {
			t.Fatal(err)
		}

		if len(charts) != 0 {
			t.Fatalf("expected zero charts to match regex, got %d", len(charts))
		}
	}
}

func TestGetChart(t *testing.T) {
	if TestArchiveBucket != "" {
		tr, err := NewGCSRepo("testName", TestArchiveBucket, nil)
		if err != nil {
			t.Fatal(err)
		}

		tc, err := tr.GetChart(TestArchiveName)
		if err != nil {
			t.Fatal(err)
		}

		have := tc.Chartfile()
		want, err := chart.LoadChartfile(TestChartFile)
		if err != nil {
			t.Fatal(err)
		}

		if reflect.DeepEqual(want, have) {
			t.Fatalf("retrieved an invalid chart\nwant:%#v\nhave:\n%#v\n", want, have)
		}
	}
}

func TestGetChartWithInvalidName(t *testing.T) {
	var invalidURL = "https://bucket"
	_, err := NewGCSRepo("testName", invalidURL, nil)
	if err == nil {
		t.Fatalf("expected error did not occur for invalid URL")
	}
}
