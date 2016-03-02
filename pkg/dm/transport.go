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

package dm

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
)

type debugTransport struct {
	// Writer is the logging destination
	Writer io.Writer

	http.RoundTripper
}

// NewDebugTransport returns a debugging implementation of a RoundTripper.
func NewDebugTransport(rt http.RoundTripper) http.RoundTripper {
	return debugTransport{
		RoundTripper: rt,
		Writer:       os.Stderr,
	}
}

func (tr debugTransport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := tr.transport().(canceler); ok {
		cr.CancelRequest(req)
	}
}

func (tr debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr.logRequest(req)
	resp, err := tr.transport().RoundTrip(req)
	if err != nil {
		return nil, err
	}
	tr.logResponse(resp)
	return resp, err
}

func (tr debugTransport) transport() http.RoundTripper {
	if tr.RoundTripper != nil {
		return tr.RoundTripper
	}
	return http.DefaultTransport
}

func (tr debugTransport) logRequest(req *http.Request) {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		fmt.Fprintf(tr.Writer, "%s: %s\n", "could not dump request", err)
	}
	fmt.Fprint(tr.Writer, string(dump))
}

func (tr debugTransport) logResponse(resp *http.Response) {
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		fmt.Fprintf(tr.Writer, "%s: %s\n", "could not dump response", err)
	}
	fmt.Fprint(tr.Writer, string(dump))
}
