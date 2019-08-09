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

package monocular

import (
	"errors"
	"net/url"
)

// ErrHostnameNotProvided indicates the url is missing a hostname
var ErrHostnameNotProvided = errors.New("no hostname provided")

// Client represents a client capable of communicating with the Monocular API.
type Client struct {

	// The base URL for requests
	BaseURL string

	// The internal logger to use
	Log func(string, ...interface{})
}

// New creates a new client
func New(u string) (*Client, error) {

	// Validate we have a URL
	if err := validate(u); err != nil {
		return nil, err
	}

	return &Client{
		BaseURL: u,
		Log:     nopLogger,
	}, nil
}

var nopLogger = func(_ string, _ ...interface{}) {}

// Validate if the base URL for monocular is valid.
func validate(u string) error {

	// Check if it is parsable
	p, err := url.Parse(u)
	if err != nil {
		return err
	}

	// Check that a host is attached
	if p.Hostname() == "" {
		return ErrHostnameNotProvided
	}

	return nil
}
