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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/ghodss/yaml"
)

// ConvertURLsToStrings converts a slice of *url.URL to a slice of string.
func ConvertURLsToStrings(urls []*url.URL) []string {
	var result []string
	for _, u := range urls {
		result = append(result, u.String())
	}

	return result
}

// TrimURLScheme removes the scheme, if any, from an URL.
func TrimURLScheme(URL string) string {
	parts := strings.SplitAfter(URL, "://")
	if len(parts) > 1 {
		return parts[1]
	}

	return URL
}

// A HandlerTester is a function that takes an HTTP method, an URL path, and a
// reader for a request body, creates a request from them, and serves it to the
// handler to which it was bound and returns a response recorder describing the
// outcome.
type HandlerTester func(method, path, ctype string, reader io.Reader) (*httptest.ResponseRecorder, error)

// NewHandlerTester creates and returns a new HandlerTester for an http.Handler.
func NewHandlerTester(handler http.Handler) HandlerTester {
	return func(method, path, ctype string, reader io.Reader) (*httptest.ResponseRecorder, error) {
		r, err := http.NewRequest(method, path, reader)
		if err != nil {
			return nil, err
		}

		r.Header.Set("Content-Type", ctype)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w, nil
	}
}

// A ServerTester is a function that takes an HTTP method, an URL path, and a
// reader for a request body, creates a request from them, and serves it to a
// test server using the handler to which it was bound and returns the response.
type ServerTester func(method, path, ctype string, reader io.Reader) (*http.Response, error)

// NewServerTester creates and returns a new NewServerTester for an http.Handler.
func NewServerTester(handler http.Handler) ServerTester {
	return func(method, path, ctype string, reader io.Reader) (*http.Response, error) {
		server := httptest.NewServer(handler)
		defer server.Close()
		request := fmt.Sprintf("%s/%s", server.URL, path)
		r, err := http.NewRequest(method, request, reader)
		if err != nil {
			return nil, err
		}

		r.Header.Set("Content-Type", ctype)
		return http.DefaultClient.Do(r)
	}
}

const formContentType = "application/x-www-form-urlencoded; param=value"

// TestWithURL invokes a HandlerTester with the given HTTP method, an URL path
// parsed from the given URL string, and a string reader on the query parameters
// parsed from the given URL string.
func (h HandlerTester) TestWithURL(method, urlString string) (*httptest.ResponseRecorder, error) {
	request, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	reader := strings.NewReader(request.Query().Encode())
	return h(method, request.Path, formContentType, reader)
}

// TestHandlerWithURL creates a HandlerTester with the given handler, and tests
// it with the given HTTP method and URL string using HandlerTester.TestWithURL.
func TestHandlerWithURL(handler http.Handler, method, urlString string) (*httptest.ResponseRecorder, error) {
	return NewHandlerTester(handler).TestWithURL(method, urlString)
}

// LogHandlerEntry logs the start of the given handler handling the given request.
func LogHandlerEntry(handler string, r *http.Request) {
	log.Printf("%s: handling request:%s %s\n", handler, r.Method, r.URL.RequestURI())
}

// LogHandlerExit logs the response from the given handler with the given results.
func LogHandlerExit(handler string, statusCode int, status string, w http.ResponseWriter) {
	log.Printf("%s: returning response: status code:%d, status:%s\n", handler, statusCode, status)
}

// LogAndReturnError logs the given error and status to stderr,
// and then returns them as the HTTP response.
func LogAndReturnError(handler string, statusCode int, err error, w http.ResponseWriter) {
	LogHandlerExit(handler, statusCode, err.Error(), w)
	http.Error(w, err.Error(), statusCode)
}

// LogHandlerExitWithText converts the given string to []byte,
// writes it to the response body, returns the given status,
// and then logs the response
func LogHandlerExitWithText(handler string, w http.ResponseWriter, v string, statusCode int) {
	msg := []byte(v)
	WriteResponse(handler, w, msg, "text/plain; charset=UTF-8", statusCode)
	LogHandlerExit(handler, statusCode, string(msg), w)
}

// LogHandlerExitWithJSON marshals the given object as JSON,
// writes it to the response body, returns the given status, and then logs the
// response.
func LogHandlerExitWithJSON(handler string, w http.ResponseWriter, v interface{}, statusCode int) {
	j := MarshalAndWriteJSON(handler, w, v, statusCode)
	LogHandlerExit(handler, statusCode, string(j), w)
}

// MarshalAndWriteJSON marshals the given object as JSON, writes it
// to the response body, and then returns the given status.
func MarshalAndWriteJSON(handler string, w http.ResponseWriter, v interface{}, statusCode int) []byte {
	j, err := json.Marshal(v)
	if err != nil {
		LogAndReturnError(handler, http.StatusInternalServerError, err, w)
		return nil
	}

	WriteJSON(handler, w, j, statusCode)
	return j
}

// WriteJSON writes the given bytes to the response body, sets the content type
// to "application/json; charset=UTF-8", and then returns the given status.
func WriteJSON(handler string, w http.ResponseWriter, j []byte, status int) {
	WriteResponse(handler, w, j, "application/json; charset=UTF-8", status)
}

// LogHandlerExitWithYAML marshals the given object as YAML,
// writes it to the response body, returns the given status, and then logs the
// response.
func LogHandlerExitWithYAML(handler string, w http.ResponseWriter, v interface{}, statusCode int) {
	y := MarshalAndWriteYAML(handler, w, v, statusCode)
	LogHandlerExit(handler, statusCode, string(y), w)
}

// MarshalAndWriteYAML marshals the given object as YAML, writes it
// to the response body, and then returns the given status.
func MarshalAndWriteYAML(handler string, w http.ResponseWriter, v interface{}, statusCode int) []byte {
	y, err := yaml.Marshal(v)
	if err != nil {
		LogAndReturnError(handler, http.StatusInternalServerError, err, w)
		return nil
	}

	WriteYAML(handler, w, y, statusCode)
	return y
}

// WriteYAML writes the given bytes to the response body, sets the content type
// to "application/x-yaml; charset=UTF-8", and then returns the given status.
func WriteYAML(handler string, w http.ResponseWriter, y []byte, status int) {
	WriteResponse(handler, w, y, "application/x-yaml; charset=UTF-8", status)
}

// WriteResponse writes the given bytes to the response body, sets the content
// type to the given value, and then returns the given status.
func WriteResponse(handler string, w http.ResponseWriter, v []byte, ct string, status int) {
	// Header must be set before status is written
	if len(v) > 0 {
		w.Header().Set("Content-Type", ct)
	}

	// Header and status must be written before content is written
	w.WriteHeader(status)
	if len(v) > 0 {
		if _, err := w.Write(v); err != nil {
			LogAndReturnError(handler, http.StatusInternalServerError, err, w)
		}
	}
}

// ToYAMLOrError marshals the given object to YAML and returns either the
// resulting YAML or an error message. Useful when marshaling an object for
// a log entry.
func ToYAMLOrError(v interface{}) string {
	y, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprintf("yaml marshal failed:%s\n%v\n", err, v)
	}

	return string(y)
}

// ToJSONOrError marshals the given object to JSON and returns either the
// resulting YAML or an error message. Useful when marshaling an object for
// a log entry.
func ToJSONOrError(v interface{}) string {
	j, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("json marshal failed:%s\n%v\n", err, v)
	}

	return string(j)
}

// IsHttpURL returns whether a string is an HTTP URL.
func IsHttpUrl(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	return u.Scheme == "http" || u.Scheme == "https"
}
