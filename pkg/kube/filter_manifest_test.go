/*
Copyright The Helm Authors.

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

package kube

import (
	"bytes"
	"io"
	"testing"
	"testing/iotest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testCase struct {
	name   string
	input  string
	output string
}

var testCases = []testCase{
	{
		name: "testcase 1",
		input: `
apiVersion: v1
kind: Namespace
--- # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---
apiVersion: test/v9
kind: Test
---`,
		output: ` # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---`,
	},
	{
		name: "testcase 2",
		input: `
apiVersion: v1
kind: Namespace
--- # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---
apiVersion: test/v9
kind: Test
---
# llllll
aaa: whut

`,
		output: ` # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---`,
	},
	{
		name: "testcase 3",
		input: `
apiVersion: v1
kind: Namespace
--- # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---
apiVersion: test/v9
kind: Test
---
# llllll
aaa: aaaaaaaaaaa

`,
		output: ` # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---`,
	},
	{
		name: "testcase 4",
		input: `
apiVersion: v1
kind: Namespace
--- # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---
apiVersion: test/v9
kind: Test
---`,
		output: ` # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value
---`,
	},
	{
		name: "testcase 5",
		input: `
apiVersion: v1
kind: Namespace
--- # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value---
---
apiVersion: test/v9
kind: Test
---
apiVersion: test/v1
kind: Test
---
apiVersion: test/v1
kind: Test
---
---
--
-----
---
apiVersion: test/v1
kind: Test
---`,
		output: ` # aaaa
# aaaaa
apiVersion: test/v1
kind: Test
field: value---
---
apiVersion: test/v1
kind: Test
---
apiVersion: test/v1
kind: Test
---
apiVersion: test/v1
kind: Test
---`,
	},
	{
		name: "testcase 6",
		input: `apiVersion: test/v1
kind: Test`,
		output: `apiVersion: test/v1
kind: Test`,
	},
}

func testTestCase(t *testing.T, initialBufferSize int, input io.Reader, output []byte) {
	t.Helper()

	reader := filterManifest(io.NopCloser(input), func(err error, o *metav1.PartialObjectMetadata) bool {
		return (err == nil) && (o.GroupVersionKind().Kind == "Test" &&
			o.GroupVersionKind().Version == "v1" &&
			o.GroupVersionKind().Group == "test")
	}, initialBufferSize, 9999999)

	err := iotest.TestReader(reader, output)
	if err != nil {
		t.Error(err)
	}
}

func TestAll(t *testing.T) {
	for _, test := range testCases {
		t.Log(test.name)
		testTestCase(t, 10, bytes.NewBufferString(test.input), []byte(test.output))
		testTestCase(t, 10, iotest.DataErrReader(bytes.NewBufferString(test.input)), []byte(test.output))
		testTestCase(t, 10, iotest.HalfReader(bytes.NewBufferString(test.input)), []byte(test.output))
		testTestCase(t, 10, iotest.OneByteReader(bytes.NewBufferString(test.input)), []byte(test.output))
	}
}

func FuzzBufferSize(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(2)
	f.Add(9)
	f.Add(10000)
	f.Fuzz(func(t *testing.T, initialBuffSize int) {
		if initialBuffSize < 50*1024*1024 && initialBuffSize >= 0 {
			for _, test := range testCases {
				t.Log(test.name)
				testTestCase(t, initialBuffSize, bytes.NewBufferString(test.input), []byte(test.output))
				testTestCase(t, initialBuffSize, iotest.DataErrReader(bytes.NewBufferString(test.input)), []byte(test.output))
				testTestCase(t, initialBuffSize, iotest.HalfReader(bytes.NewBufferString(test.input)), []byte(test.output))
				testTestCase(t, initialBuffSize, iotest.OneByteReader(bytes.NewBufferString(test.input)), []byte(test.output))
			}
		}
	})
}
