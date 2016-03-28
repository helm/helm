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

// ExpanderResponse gives back a layout, which has nested structure
// Resource0
//		ResourceDefinition
//		Resource0, 0
//				ResourceDefinition
//				Resource0, 0, 0
//						ResourceDefinition
//				Resource0, 0, 1
//						ResourceDefinition
//		Resource0, 1
//				ResourceDefinition
//
// All the leaf nodes in this tree are either primitives or a currently unexpandable type.
// Next we will resolve all the unexpandable types and re-enter expansion, at which point
// all primitives are untouched and returned as root siblings with no children in the
// resulting layout. The previously unexpandable nodes will become sibling root nodes,
// but with children. We want to replace the leaf nodes that were formerly unexpandable
// with their respective newly created trees.
//
// So, do as follows:
// 1) Do a walk of the tree and find each leaf. Check its Type and place a pointer to it
// into a map with the resource name and type as key if it is non-primitive.
// 2) Re-expand the template with the new imports.
// 3) For each root level sibling, check if its name exists in the hash map from (1)
// 4) Replace the Layout of the node in the hash map with the current node if applicable.
// 5) Return to (1)

// TODO (iantw): There may be a tricky corner case here where a known template could be
// masked by an unknown template, which on the subsequent expansion could allow a collision
// between the name#template key to exist in the layout given a particular choice of naming.
// In practice, it would be nearly impossible to hit, but consider including properties/name/type
// into a hash of sorts to make this robust...
/*
func walkLayout(l *common.Layout, imports []*common.ImportFile, toReplace map[string]*common.LayoutResource) map[string]*common.LayoutResource {
	ret := map[string]*common.LayoutResource{}
	toVisit := l.Resources

	for len(toVisit) > 0 {
		lr := toVisit[0]
		nodeKey := lr.Resource.Name + layoutNodeKeySeparator + lr.Resource.Type
		if len(lr.Layout.Resources) == 0 && isTemplate(lr.Resource.Type, imports) {
			ret[nodeKey] = lr
		} else if toReplace[nodeKey] != nil {
			toReplace[nodeKey].Resources = lr.Resources
		}
		toVisit = append(toVisit, lr.Resources...)
		toVisit = toVisit[1:]
	}

	return ret
}
*/

// ExpandConfiguration expands the supplied configuration and returns
// an expanded configuration.
func (e *expander) ExpandConfiguration(conf *common.Configuration) (*ExpandedConfiguration, error) {
	expConf, err := e.expandConfiguration(conf)
	if err != nil {
		return nil, fmt.Errorf("cannot expand configuration:%s\n%v\n", err, conf)
	}

	return expConf, nil
}

func (e *expander) expandConfiguration(conf *common.Configuration) (*ExpandedConfiguration, error) {
	resources := []*common.Resource{}
	layout := []*common.LayoutResource{}

	for _, resource := range conf.Resources {
		if !repo.IsChartReference(resource.Type) {
			resources = append(resources, resource)
			continue
		}

		cbr, _, err := e.repoProvider.GetChartByReference(resource.Type)
		if err != nil {
			return nil, err
		}

		defer cbr.Close()
		content, err := cbr.LoadContent()
		if err != nil {
			return nil, err
		}

		svcReq := &expansion.ServiceRequest{
			ChartInvocation: resource,
			Chart:           content,
		}

		svcResp, err := e.callService(svcReq)
		if err != nil {
			return nil, err
		}

		expConf, err := e.expandConfiguration(svcResp)
		if err != nil {
			return nil, err
		}

		// TODO: build up layout hiearchically
		resources = append(resources, expConf.Config.Resources...)
		layout = append(layout, expConf.Layout.Resources...)
	}

	return &ExpandedConfiguration{
		Config: &common.Configuration{Resources: resources},
		Layout: &common.Layout{Resources: layout},
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
