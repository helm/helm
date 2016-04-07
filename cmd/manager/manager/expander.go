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
	"github.com/kubernetes/helm/pkg/util"

	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

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
func NewExpander(port, URL string, rp repo.IRepoProvider) Expander {
	if rp == nil {
		rp = repo.NewRepoProvider(nil, nil, nil)
	}

	return &expander{expanderPort: port, expanderURL: URL, repoProvider: rp}
}

type expander struct {
	repoProvider repo.IRepoProvider
	expanderPort string
	expanderURL  string
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
		additions := []*common.Resource{resource}
		layout := &common.LayoutResource{
			Resource: common.Resource{
				Name: resource.Name, Type: resource.Type,
			},
		}

		// If the type is a chart reference
		if repo.IsChartReference(resource.Type) {
			// Fetch, decompress and unpack
			cbr, _, err := e.repoProvider.GetChartByReference(resource.Type)
			if err != nil {
				return nil, err
			}

			defer cbr.Close()

			// Load the charts contents into strings that we can pass to exapnsion
			content, err := cbr.LoadContent()
			if err != nil {
				return nil, err
			}

			expander := cbr.Chartfile().Expander
			if expander != nil && expander.Name != "" {
				// Build a request to the expansion service and call it to do the expansion
				svcReq := &expansion.ServiceRequest{
					ChartInvocation: resource,
					Chart:           content,
				}

				svcResp, err := e.callService(expander.Name, svcReq)
				if err != nil {
					return nil, err
				}

				// Call ourselves recursively with the list of resources returned by expansion
				expConf, err := e.expandConfiguration(svcResp)
				if err != nil {
					return nil, err
				}

				// Append the reources returned by the recursion to the flat list of resources
				additions = expConf.Config.Resources

				// This was not a primitive resource, so add its properties to the layout
				// Then add the all of the layout resources returned by the recursion to the layout
				layout.Resources = expConf.Layout.Resources
				layout.Properties = resource.Properties
			} else {
				// Raise an error if a non template chart supplies properties
				if resource.Properties != nil {
					return nil, fmt.Errorf("properties provided for non template chart %s", resource.Type)
				}

				additions = []*common.Resource{}
				for _, member := range content.Members {
					segments := strings.Split(member.Path, "/")
					if len(segments) > 1 && segments[0] == "templates" {
						if strings.HasSuffix(member.Path, "yaml") || strings.HasSuffix(member.Path, "json") {
							resource, err := util.ParseKubernetesObject(member.Content)
							if err != nil {
								return nil, err
							}

							resources = append(resources, resource)
						}
					}
				}
			}
		}

		resources = append(resources, additions...)
		layouts = append(layouts, layout)
	}

	// All done with this level, so return the expanded configuration
	result := &ExpandedConfiguration{
		Config: &common.Configuration{Resources: resources},
		Layout: &common.Layout{Resources: layouts},
	}

	return result, nil
}

func (e *expander) callService(svcName string, svcReq *expansion.ServiceRequest) (*common.Configuration, error) {
	svcURL, err := e.getServiceURL(svcName)
	if err != nil {
		return nil, err
	}

	j, err := json.Marshal(svcReq)
	if err != nil {
		return nil, err
	}

	reader := ioutil.NopCloser(bytes.NewReader(j))
	request, err := http.NewRequest("POST", svcURL, reader)
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

func (e *expander) getServiceURL(svcName string) (string, error) {
	if !strings.HasPrefix(svcName, "http:") && !strings.HasPrefix(svcName, "https:") {
		var err error
		svcName, err = util.GetServiceURL(svcName, e.expanderPort, e.expanderURL)
		if err != nil {
			return "", err
		}
	}

	u, err := url.Parse(svcName)
	if err != nil {
		return "", err
	}

	u.Path = fmt.Sprintf("%s/expand", u.Path)
	return u.String(), nil
}
