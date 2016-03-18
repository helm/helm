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
	"io/ioutil"
	"net/http"
	"os"
	fancypath "path"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/helm/pkg/common"
)

// ListDeployments lists the deployments in DM.
func (c *Client) ListDeployments() ([]string, error) {
	var l []string
	_, err := c.Get("deployments", &l)
	return l, err
}

// PostChart sends a chart to DM for deploying.
//
// This returns the location for the new chart, typically of the form
// `helm:repo/bucket/name-version.tgz`.
func (c *Client) PostChart(filename, deployname string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}

	u, err := c.url("/v2/charts")
	request, err := http.NewRequest("POST", u, f)
	if err != nil {
		f.Close()
		return "", err
	}

	// There is an argument to be made for using the legacy x-octet-stream for
	// this. But since we control both sides, we should use the standard one.
	// Also, gzip (x-compress) is usually treated as a content encoding. In this
	// case it probably is not, but it makes more sense to follow the standard,
	// even though we don't assume the remote server will strip it off.
	request.Header.Add("Content-Type", "application/x-tar")
	request.Header.Add("Content-Encoding", "gzip")
	request.Header.Add("X-Deployment-Name", deployname)
	request.Header.Add("X-Chart-Name", filepath.Base(filename))
	request.Header.Set("User-Agent", c.agent())

	client := &http.Client{
		Timeout:   c.HTTPTimeout,
		Transport: c.transport(),
	}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	// We only want 201 CREATED. Admittedly, we could accept 200 and 202.
	if response.StatusCode != http.StatusCreated {
		body, err := ioutil.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return "", err
		}
		return "", &HTTPError{StatusCode: response.StatusCode, Message: string(body), URL: request.URL}
	}

	loc := response.Header.Get("Location")
	return loc, nil
}

// GetDeployment retrieves the supplied deployment
func (c *Client) GetDeployment(name string) (*common.Deployment, error) {
	var deployment *common.Deployment
	_, err := c.Get(fancypath.Join("deployments", name), &deployment)
	return deployment, err
}

// DeleteDeployment deletes the supplied deployment
func (c *Client) DeleteDeployment(name string) (*common.Deployment, error) {
	var deployment *common.Deployment
	_, err := c.Delete(filepath.Join("deployments", name), &deployment)
	return deployment, err
}

// PostDeployment posts a deployment object to the manager service.
func (c *Client) PostDeployment(name string, cfg *common.Configuration) error {
	d, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	// This is a stop-gap until we get this API cleaned up.
	t := common.Template{
		Name:    name,
		Content: string(d),
	}

	data, err := json.Marshal(t)
	if err != nil {
		return err
	}

	var out struct{}
	_, err = c.Post("/deployments", data, &out)
	return err
}
