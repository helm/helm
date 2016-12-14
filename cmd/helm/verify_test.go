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

package main

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"
)

func TestVerifyCmd(t *testing.T) {

	statExe := "stat"
	statPathMsg := "no such file or directory"
	statFileMsg := statPathMsg
	if runtime.GOOS == "windows" {
		statExe = "GetFileAttributesEx"
		statPathMsg = "The system cannot find the path specified."
		statFileMsg = "The system cannot find the file specified."
	}

	tests := []struct {
		name   string
		args   []string
		flags  []string
		expect string
		err    bool
	}{
		{
			name:   "verify requires a chart",
			expect: "a path to a package file is required",
			err:    true,
		},
		{
			name:   "verify requires that chart exists",
			args:   []string{"no/such/file"},
			expect: fmt.Sprintf("%s no/such/file: %s", statExe, statPathMsg),
			err:    true,
		},
		{
			name:   "verify requires that chart is not a directory",
			args:   []string{"testdata/testcharts/signtest"},
			expect: "unpacked charts cannot be verified",
			err:    true,
		},
		{
			name:   "verify requires that chart has prov file",
			args:   []string{"testdata/testcharts/compressedchart-0.1.0.tgz"},
			expect: fmt.Sprintf("could not load provenance file testdata/testcharts/compressedchart-0.1.0.tgz.prov: %s testdata/testcharts/compressedchart-0.1.0.tgz.prov: %s", statExe, statFileMsg),
			err:    true,
		},
		{
			name:   "verify validates a properly signed chart",
			args:   []string{"testdata/testcharts/signtest-0.1.0.tgz"},
			flags:  []string{"--keyring", "testdata/helm-test-key.pub"},
			expect: "",
			err:    false,
		},
	}

	for _, tt := range tests {
		b := bytes.NewBuffer(nil)
		vc := newVerifyCmd(b)
		vc.ParseFlags(tt.flags)
		err := vc.RunE(vc, tt.args)
		if tt.err {
			if err == nil {
				t.Errorf("Expected error, but got none: %q", b.String())
			}
			if err.Error() != tt.expect {
				t.Errorf("Expected error %q, got %q", tt.expect, err)
			}
			continue
		} else if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
		if b.String() != tt.expect {
			t.Errorf("Expected %q, got %q", tt.expect, b.String())
		}
	}
}
