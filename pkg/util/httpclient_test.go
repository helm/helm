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

package util

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type mockSleeper struct {
	args []time.Duration
}

func (m *mockSleeper) Sleep(d time.Duration) {
	m.args = append(m.args, d)
}

type responseAndError struct {
	err  error
	resp *http.Response
}

type testBody struct {
	closed bool
	body   io.Reader
}

func (tb *testBody) Read(p []byte) (n int, err error) {
	return tb.body.Read(p)
}

func (tb *testBody) Close() error {
	tb.closed = true
	return nil
}

func createResponse(err error, code int, body string, shouldClose bool,
	headers map[string]string) responseAndError {
	httpBody := testBody{body: strings.NewReader(body), closed: !shouldClose}
	header := http.Header{}
	for k, v := range headers {
		header.Add(k, v)
	}
	httpResponse := &http.Response{
		Body:          &httpBody,
		ContentLength: int64(len(body)),
		StatusCode:    code,
		Header:        header,
	}
	return responseAndError{err: err, resp: httpResponse}
}

type mockDoer struct {
	resp    []responseAndError
	t       *testing.T
	url     string
	headers map[string]string
}

func (doer *mockDoer) Do(req *http.Request) (res *http.Response, err error) {
	if req.URL.String() != doer.url {
		doer.t.Errorf("Expected url %s but got url %s", doer.url, req.URL.String())
	}

	for k, v := range doer.headers {
		if req.Header.Get(k) != v {
			doer.t.Errorf("Expected header %s with value %s but found %s", k, v, req.Header.Get(k))
		}
	}

	if len(doer.resp) == 0 {
		doer.t.Errorf("Do method was called more times than expected.")
	}

	res = doer.resp[0].resp
	err = doer.resp[0].err
	doer.resp = doer.resp[1:]
	return
}

func testClientDriver(md mockDoer, ms mockSleeper, expectedErr error, code int,
	result string, t *testing.T) {
	expectedCalls := len(md.resp)
	client := NewHTTPClient(uint(expectedCalls)-1, &md, &ms)

	r, c, e := client.Get(md.url)

	if expectedCalls-1 != len(ms.args) {
		t.Errorf("Expected %d calls to sleeper but found %d", expectedCalls-1, len(ms.args))
	}

	if r != result {
		t.Errorf("Expected result %s but received %s", result, r)
	}

	if c != code {
		t.Errorf("Expected status code %d but received %d", code, c)
	}

	if e != expectedErr {
		t.Errorf("Expected error %s but received %s", expectedErr, e)
	}
}

func TestGzip(t *testing.T) {
	doer := mockDoer{}
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	gz.Write([]byte("Test"))
	gz.Flush()
	gz.Close()
	result := b.String()

	doer.resp = []responseAndError{
		createResponse(nil, 200, result, true, map[string]string{"Content-Encoding": "gzip"}),
	}

	sleeper := mockSleeper{}
	testClientDriver(doer, sleeper, nil, 200, "Test", t)
}

func TestRetry(t *testing.T) {
	doer := mockDoer{}
	doer.resp = []responseAndError{
		createResponse(nil, 404, "", true, map[string]string{}),
		createResponse(nil, 200, "Test", true, map[string]string{}),
	}

	sleeper := mockSleeper{}
	testClientDriver(doer, sleeper, nil, 200, "Test", t)
}

func TestFail(t *testing.T) {
	doer := mockDoer{}
	err := errors.New("Error")
	doer.resp = []responseAndError{
		createResponse(nil, 404, "", true, map[string]string{}),
		createResponse(err, 0, "", false, map[string]string{}),
	}

	sleeper := mockSleeper{}
	testClientDriver(doer, sleeper, err, 0, "", t)
}
