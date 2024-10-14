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
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type RetryingRoundTripper struct {
	Wrapped http.RoundTripper
}

func (rt *RetryingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.roundTrip(req, 1, nil)
}

func (rt *RetryingRoundTripper) roundTrip(req *http.Request, retry int, prevResp *http.Response) (*http.Response, error) {
	if retry < 0 {
		return prevResp, nil
	}
	resp, rtErr := rt.Wrapped.RoundTrip(req)
	if rtErr != nil {
		return resp, rtErr
	}
	if resp.StatusCode < 500 {
		return resp, rtErr
	}
	if resp.Header.Get("content-type") != "application/json" {
		return resp, rtErr
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return resp, rtErr
	}

	var ke kubernetesError
	r := bytes.NewReader(b)
	err = json.NewDecoder(r).Decode(&ke)
	r.Seek(0, io.SeekStart)
	resp.Body = io.NopCloser(r)
	if err != nil {
		return resp, rtErr
	}
	if ke.Code < 500 {
		return resp, rtErr
	}
	// Matches messages like "etcdserver: leader changed"
	if strings.HasSuffix(ke.Message, "etcdserver: leader changed") {
		return rt.roundTrip(req, retry-1, resp)
	}
	// Matches messages like "rpc error: code = Unknown desc = raft proposal dropped"
	if strings.HasSuffix(ke.Message, "raft proposal dropped") {
		return rt.roundTrip(req, retry-1, resp)
	}
	return resp, rtErr
}

type kubernetesError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}
