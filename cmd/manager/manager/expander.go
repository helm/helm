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
	"github.com/kubernetes/helm/pkg/common"
	"github.com/kubernetes/helm/pkg/expansion"
	"github.com/kubernetes/helm/pkg/repo"

	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

/*
const (
	// TODO (iantw): Align this with a character not allowed to show up in resource names.
	layoutNodeKeySeparator = "#"
)
*/

// ExpandedConfiguration is the structure returned by the expansion service.
type ExpandedConfiguration struct {
	Config *common.Configuration `json:"config"`
	Layout *common.Layout        `json:"layout"`
}

// Expander abstracts interactions with the expander and deployer services.
type Expander interface {
	ExpandConfiguration(conf *common.Configuration) (*ExpandedConfiguration, error)
}

// NewExpander returns a new initialized Expander.
func NewExpander(URL string, rp repo.IRepoProvider) Expander {
	if rp == nil {
		rp = repo.NewRepoProvider(nil, nil, nil)
	}

	return &expander{expanderURL: URL, repoProvider: rp}
}

type expander struct {
	repoProvider repo.IRepoProvider
	expanderURL  string
}

func (e *expander) getBaseURL() string {
	return fmt.Sprintf("%s/expand", e.expanderURL)
}

// ExpandConfiguration expands the supplied configuration and returns
// an expanded configuration.
func (e *expander) ExpandConfiguration(conf *common.Configuration) (*ExpandedConfiguration, error) {
	expConf, err := e.expandConfiguration(conf)
	if err != nil {
		return nil, err
	}

	return expConf, nil
}

func (e *expander) expandConfiguration(conf *common.Configuration) (*ExpandedConfiguration, error) {
	resources := []*common.Resource{}
	layouts := []*common.LayoutResource{}

	// Iterate over all of the resources in the unexpanded configuration
	for _, resource := range conf.Resources {
		// A primitive layout resource captures only the name and type
		layout := &common.LayoutResource{
			Resource: common.Resource{
				Name: resource.Name, Type: resource.Type,
			},
		}

		// If the type is not a chart reference, then it must be primitive
		if !repo.IsChartReference(resource.Type) {
			// Add it to the flat list of exapnded resources
			resources = append(resources, resource)

			// Add its layout to the list of layouts at this level
			layouts = append(layouts, layout)
			continue
		}

		// It is a chart, so go fetch it, decompress it and unpack it
		cbr, _, err := e.repoProvider.GetChartByReference(resource.Type)
		if err != nil {
			return nil, err
		}

		defer cbr.Close()

		// Now, load the charts contents into strings that we can pass to exapnsion
		content, err := cbr.LoadContent()
		if err != nil {
			return nil, err
		}

		// Build a request to the expansion service and call it to do the expansion
		svcReq := &expansion.ServiceRequest{
			ChartInvocation: resource,
			Chart:           content,
		}

		svcResp, err := e.callService(svcReq)
		if err != nil {
			return nil, err
		}

		// Call ourselves recursively with the list of resources returned by expansion
		expConf, err := e.expandConfiguration(svcResp)
		if err != nil {
			return nil, err
		}

		// Append the reources returned by the recursion to the flat list of resources
		resources = append(resources, expConf.Config.Resources...)

		// This was not a primitive resource, so add its properties to the layout
		layout.Properties = resource.Properties

		// Now add the all of the layout resources returned by the recursion to the layout
		layout.Resources = expConf.Layout.Resources
		layouts = append(layouts, layout)
	}

	// All done with this level, so return the espanded configuration
	return &ExpandedConfiguration{
		Config: &common.Configuration{Resources: resources},
		Layout: &common.Layout{Resources: layouts},
	}, nil
}

func (e *expander) callService(svcReq *expansion.ServiceRequest) (*common.Configuration, error) {
	j, err := json.Marshal(svcReq)
	if err != nil {
		return nil, err
	}

	reader := ioutil.NopCloser(bytes.NewReader(j))
	request, err := http.NewRequest("POST", e.getBaseURL(), reader)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "*/*")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		e := fmt.Errorf("call failed (%s) with payload:\n%s\n", err, string(j))
		return nil, e
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		e := fmt.Errorf("error reading response: %s", err)
		return nil, e
	}

	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("expandybird response:\n%s", body)
		return nil, err
	}

	svcResp := &common.Configuration{}
	if err := json.Unmarshal(body, svcResp); err != nil {
		e := fmt.Errorf("cannot unmarshal response body (%s):%s", err, body)
		return nil, e
	}

	return svcResp, nil
}
