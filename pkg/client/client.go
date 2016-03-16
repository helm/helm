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

// DefaultHTTPTimeout is the default HTTP timeout.
var DefaultHTTPTimeout = time.Second * 10

// DefaultHTTPProtocol is the default HTTP Protocol (http, https).
var DefaultHTTPProtocol = "http"

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

// CallService is a low-level function for making an API call.
//
// This calls the service and then unmarshals the returned data into dest.
func (c *Client) CallService(path, method, action string, dest interface{}, reader io.Reader) error {
	u, err := c.url(path)
	if err != nil {
		return err
	}

	resp, err := c.callHTTP(u, method, action, reader)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(resp), dest); err != nil {
		return fmt.Errorf("Failed to parse JSON response from service: %s", resp)
	}
	return nil
}

// callHTTP is  a low-level primitive for executing HTTP operations.
func (c *Client) callHTTP(path, method, action string, reader io.Reader) (string, error) {
	request, err := http.NewRequest(method, path, reader)

	// TODO: dynamically set version
	request.Header.Set("User-Agent", c.agent())
	request.Header.Add("Content-Type", "application/json")

	client := &http.Client{
		Timeout:   c.HTTPTimeout,
		Transport: c.transport(),
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	s := response.StatusCode
	if s < http.StatusOK || s >= http.StatusMultipleChoices {
		return "", &HTTPError{StatusCode: s, Message: string(body), URL: request.URL}
	}

	return string(body), nil
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
