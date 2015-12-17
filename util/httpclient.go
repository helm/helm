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
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	accHeader  = "Accept-Encoding"
	typeHeader = "Content-Type"
	encHeader  = "Content-Encoding"
	gzipHeader = "gzip"
)

// TODO (iantw): Consider creating the Duration objects up front... May just need an all around
// refactor if we want to support other types of backoff etc.
var sleepIntervals = []int{1, 1, 2, 3, 5, 8, 10}

// Sleeper exposes a Sleep func which causes the current goroutine to sleep for the requested
// duration.
type Sleeper interface {
	Sleep(d time.Duration)
}

type sleeper struct {
}

func (s sleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// NewSleeper returns a new Sleeper.
func NewSleeper() Sleeper {
	return sleeper{}
}

// HTTPDoer is an interface for something that can 'Do' an http.Request and return an http.Response
// and error.
type HTTPDoer interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// HTTPClient is a higher level HTTP client which takes a URL and returns the response body as a
// string, along with the resulting status code and any errors.
type HTTPClient interface {
	Get(url string) (body string, code int, err error)
}

type httpClient struct {
	retries uint
	client  HTTPDoer
	sleep   Sleeper
}

// NewHTTPClient returns a new HTTPClient.
func NewHTTPClient(retries uint, c HTTPDoer, s Sleeper) HTTPClient {
	ret := httpClient{}
	ret.client = c
	ret.sleep = s
	ret.retries = retries
	return ret
}

func readBody(b io.ReadCloser, ctype string, encoding string) (body string, err error) {
	defer b.Close()
	var r io.Reader
	if encoding == gzipHeader {
		gr, err := gzip.NewReader(b)
		if err != nil {
			return "", err
		}
		r = gr
		defer gr.Close()
	} else if encoding == "" {
		r = b
	} else {
		return "", fmt.Errorf("Unknown %s: %s", encHeader, encoding)
	}

	// TODO(iantw): If we find a need, allow character set conversions...
	// Unlikely to be an issue for now.
	// if ctype != "" {
	// 	 r, err = charset.NewReader(r, ctype)
	//
	//	 if err != nil {
	//		 return "", err
	//	 }
	// }

	bytes, err := ioutil.ReadAll(r)
	return string(bytes), err
}

func (client httpClient) Get(url string) (body string, code int, err error) {
	retryCount := client.retries
	numRetries := uint(0)
	shouldRetry := true
	for shouldRetry {
		body = ""
		code = 0
		err = nil
		var req *http.Request
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return "", 0, err
		}

		req.Header.Add(accHeader, gzipHeader)

		var r *http.Response
		r, err = client.client.Do(req)
		if err == nil {
			body, err = readBody(r.Body, r.Header.Get(typeHeader), r.Header.Get(encHeader))
			if err == nil {
				code = r.StatusCode
			}
		}

		if code != 200 {
			if numRetries < retryCount {
				numRetries = numRetries + 1
				sleepIndex := int(numRetries)
				if numRetries >= uint(len(sleepIntervals)) {
					sleepIndex = len(sleepIntervals) - 1
				}
				d, _ := time.ParseDuration(string(sleepIntervals[sleepIndex]) + "s")
				client.sleep.Sleep(d)
			} else {
				shouldRetry = false
			}
		} else {
			shouldRetry = false
		}
	}
	return
}
