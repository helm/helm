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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kubernetes/helm/pkg/version"
)

const (
	// DefaultHTTPTimeout is the default HTTP timeout.
	DefaultHTTPTimeout = time.Second * 10
	// DefaultHTTPProtocol is the default HTTP Protocol (http, https).
	DefaultHTTPProtocol = "http"
)

// Client is a DM client.
type Client struct {
	// Timeout on HTTP connections.
	HTTPTimeout time.Duration
	// Transport
	Transport http.RoundTripper
	// Debug enables http logging
	Debug bool
	// Base URL for remote service
	baseURL *url.URL
}

// NewClient creates a new DM client. Host name is required.
func NewClient(host string) *Client {
	url, _ := DefaultServerURL(host)

	return &Client{
		HTTPTimeout: DefaultHTTPTimeout,
		baseURL:     url,
		Transport:   http.DefaultTransport,
	}
}

// SetDebug enables debug mode which logs http
func (c *Client) SetDebug(enable bool) *Client {
	c.Debug = enable
	return c
}

// transport wraps client transport if debug is enabled
func (c *Client) transport() http.RoundTripper {
	if c.Debug {
		return NewDebugTransport(c.Transport)
	}
	return c.Transport
}

// SetTransport sets a custom Transport. Defaults to http.DefaultTransport
func (c *Client) SetTransport(tr http.RoundTripper) *Client {
	c.Transport = tr
	return c
}

// SetTimeout sets a timeout for http connections
func (c *Client) SetTimeout(seconds int) *Client {
	c.HTTPTimeout = time.Duration(seconds) * time.Second
	return c
}

// url constructs the URL.
func (c *Client) url(rawurl string) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}
	return c.baseURL.ResolveReference(u).String(), nil
}

func (c *Client) agent() string {
	return fmt.Sprintf("helm/%s", version.Version)
}

// Get calls GET on an endpoint and decodes the response
func (c *Client) Get(endpoint string, v interface{}) (*Response, error) {
	return c.Exec(c.NewRequest("GET", endpoint, nil), &v)
}

// Post calls POST on an endpoint and decodes the response
func (c *Client) Post(endpoint string, payload, v interface{}) (*Response, error) {
	return c.Exec(c.NewRequest("POST", endpoint, payload), &v)
}

// Delete calls DELETE on an endpoint and decodes the response
func (c *Client) Delete(endpoint string, v interface{}) (*Response, error) {
	return c.Exec(c.NewRequest("DELETE", endpoint, nil), &v)
}

// NewRequest creates a new client request
func (c *Client) NewRequest(method, endpoint string, payload interface{}) *Request {
	u, err := c.url(endpoint)
	if err != nil {
		return &Request{error: err}
	}

	body := prepareBody(payload)
	req, err := http.NewRequest(method, u, body)

	req.Header.Set("User-Agent", c.agent())
	req.Header.Set("Accept", "application/json")

	// TODO: set Content-Type based on body
	req.Header.Add("Content-Type", "application/json")

	return &Request{req, err}
}

func prepareBody(payload interface{}) io.Reader {
	var body io.Reader
	switch t := payload.(type) {
	default:
		//FIXME: panic is only for development
		panic(fmt.Sprintf("unexpected type %T\n", t))
	case io.Reader:
		body = t
	case []byte:
		body = bytes.NewBuffer(t)
	case nil:
	}
	return body
}

// Exec sends a request and decodes the response
func (c *Client) Exec(req *Request, v interface{}) (*Response, error) {
	return c.Result(c.Do(req), &v)
}

// Result checks status code and decodes a response body
func (c *Client) Result(resp *Response, v interface{}) (*Response, error) {
	switch {
	case resp.error != nil:
		return resp, resp
	case !resp.Success():
		return resp, resp.HTTPError()
	}
	return resp, decodeResponse(resp, v)
}

// Do send a request and returns a response
func (c *Client) Do(req *Request) *Response {
	if req.error != nil {
		return &Response{error: req}
	}

	client := &http.Client{
		Timeout:   c.HTTPTimeout,
		Transport: c.transport(),
	}
	resp, err := client.Do(req.Request)
	return &Response{resp, err}
}

// DefaultServerURL converts a host, host:port, or URL string to the default base server API path
// to use with a Client
func DefaultServerURL(host string) (*url.URL, error) {
	if host == "" {
		return nil, fmt.Errorf("host must be a URL or a host:port pair")
	}
	base := host
	hostURL, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if hostURL.Scheme == "" {
		hostURL, err = url.Parse(DefaultHTTPProtocol + "://" + base)
		if err != nil {
			return nil, err
		}
	}
	if len(hostURL.Path) > 0 && !strings.HasSuffix(hostURL.Path, "/") {
		hostURL.Path = hostURL.Path + "/"
	}

	return hostURL, nil
}

// Request wraps http.Request to include error
type Request struct {
	*http.Request
	error
}

// Response wraps http.Response to include error
type Response struct {
	*http.Response
	error
}

// Success returns true if the status code is 2xx
func (r *Response) Success() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// HTTPError creates a new HTTPError from response
func (r *Response) HTTPError() error {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return &HTTPError{
		StatusCode: r.StatusCode,
		Message:    string(body),
		URL:        r.Request.URL,
	}
}

// HTTPError is an error caused by an unexpected HTTP status code.
//
// The StatusCode will not necessarily be a 4xx or 5xx. Any unexpected code
// may be returned.
type HTTPError struct {
	StatusCode int
	Message    string
	URL        *url.URL
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return e.Message
}

// String implmenets the io.Stringer interface.
func (e *HTTPError) String() string {
	return e.Error()
}

func decodeResponse(resp *Response, v interface{}) error {
	defer resp.Body.Close()
	if resp.Body == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("Failed to parse JSON response from service")
	}
	return nil
}
