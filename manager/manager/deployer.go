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

package manager

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/ghodss/yaml"
)

// Deployer abstracts interactions with the expander and deployer services.
type Deployer interface {
	GetConfiguration(cached *Configuration) (*Configuration, error)
	CreateConfiguration(configuration *Configuration) error
	DeleteConfiguration(configuration *Configuration) error
	PutConfiguration(configuration *Configuration) error
}

// NewDeployer returns a new initialized Deployer.
func NewDeployer(url string) Deployer {
	return &deployer{url}
}

type deployer struct {
	deployerURL string
}

func (d *deployer) getBaseURL() string {
	return fmt.Sprintf("%s/configurations", d.deployerURL)
}

type formatter func(err error) error

// GetConfiguration reads and returns the actual configuration
// of the resources described by a cached configuration.
func (d *deployer) GetConfiguration(cached *Configuration) (*Configuration, error) {
	errors := &Error{}
	actual := &Configuration{}
	for _, resource := range cached.Resources {
		rtype := url.QueryEscape(resource.Type)
		rname := url.QueryEscape(resource.Name)
		url := fmt.Sprintf("%s/%s/%s", d.getBaseURL(), rtype, rname)
		body, err := d.callService("GET", url, nil, func(e error) error {
			return fmt.Errorf("cannot get configuration for resource (%s)", e)
		})
		if err != nil {
			log.Println(errors.appendError(err))
			continue
		}

		if len(body) != 0 {
			result := &Resource{Name: resource.Name, Type: resource.Type}
			if err := yaml.Unmarshal(body, &result.Properties); err != nil {
				return nil, fmt.Errorf("cannot get configuration for resource (%v)", err)
			}

			actual.Resources = append(actual.Resources, result)
		}
	}

	if len(errors.errors) > 0 {
		return nil, errors
	}

	return actual, nil
}

// CreateConfiguration deploys the set of resources described by a configuration.
func (d *deployer) CreateConfiguration(configuration *Configuration) error {
	return d.callServiceWithConfiguration("POST", "create", configuration)
}

// DeleteConfiguration deletes the set of resources described by a configuration.
func (d *deployer) DeleteConfiguration(configuration *Configuration) error {
	return d.callServiceWithConfiguration("DELETE", "delete", configuration)
}

// PutConfiguration replaces the set of resources described by a configuration.
func (d *deployer) PutConfiguration(configuration *Configuration) error {
	return d.callServiceWithConfiguration("PUT", "replace", configuration)
}

func (d *deployer) callServiceWithConfiguration(method, operation string, configuration *Configuration) error {
	callback := func(e error) error {
		return fmt.Errorf("cannot %s configuration (%s)", operation, e)
	}

	y, err := yaml.Marshal(configuration)
	if err != nil {
		return callback(err)
	}

	reader := ioutil.NopCloser(bytes.NewReader(y))
	_, err = d.callService(method, d.getBaseURL(), reader, callback)
	return err
}

func (d *deployer) callService(method, url string, reader io.Reader, callback formatter) ([]byte, error) {
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, callback(err)
	}

	if method != "GET" {
		request.Header.Add("Content-Type", "application/json")
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, callback(err)
	}

	defer response.Body.Close()
	if response.StatusCode < http.StatusOK ||
		response.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("deployer service response:\n%v\n", response)
		return nil, callback(err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, callback(err)
	}

	return body, nil
}

// Error is an error type that captures errors from the multiple calls to kubectl
// made for a single configuration.
type Error struct {
	errors []error
}

// Error returns the string value of an Error.
func (e *Error) Error() string {
	errs := []string{}
	for _, err := range e.errors {
		errs = append(errs, err.Error())
	}

	return strings.Join(errs, "\n")
}

func (e *Error) appendError(err error) error {
	e.errors = append(e.errors, err)
	return err
}
