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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/ghodss/yaml"
)

const (
	// TODO (iantw): Align this with a character not allowed to show up in resource names.
	layoutNodeKeySeparator = "#"
)

// ExpandedTemplate is the structure returned by the expansion service.
type ExpandedTemplate struct {
	Config *Configuration `json:"config"`
	Layout *Layout        `json:"layout"`
}

// Expander abstracts interactions with the expander and deployer services.
type Expander interface {
	ExpandTemplate(t Template) (*ExpandedTemplate, error)
}

// NewExpander returns a new initialized Expander.
func NewExpander(url string, tr TypeResolver) Expander {
	return &expander{url, tr}
}

type expander struct {
	expanderURL  string
	typeResolver TypeResolver
}

func (e *expander) getBaseURL() string {
	return fmt.Sprintf("%s/expand", e.expanderURL)
}

func expanderError(t *Template, err error) error {
	return fmt.Errorf("cannot expand template named %s (%s):\n%s", t.Name, err, t.Content)
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
func walkLayout(l *Layout, toReplace map[string]*LayoutResource) map[string]*LayoutResource {
	ret := map[string]*LayoutResource{}
	toVisit := l.Resources

	for len(toVisit) > 0 {
		lr := toVisit[0]
		nodeKey := lr.Resource.Name + layoutNodeKeySeparator + lr.Resource.Type
		if len(lr.Layout.Resources) == 0 && Primitives[lr.Resource.Type] == false {
			ret[nodeKey] = lr
		} else if toReplace[nodeKey] != nil {
			toReplace[nodeKey].Resources = lr.Resources
		}
		toVisit = append(toVisit, lr.Resources...)
		toVisit = toVisit[1:]
	}

	return ret
}

// ExpandTemplate expands the supplied template, and returns a configuration.
func (e *expander) ExpandTemplate(t Template) (*ExpandedTemplate, error) {
	// We have a fencepost problem here.
	// 1. Start by trying to resolve any missing templates
	// 2. Expand the configuration using all the of the imports available to us at this point
	// 3. Expansion may yield additional templates, so we run the type resolution again
	// 4. If type resolution resulted in new imports being available, return to 2.
	config := &Configuration{}
	if err := yaml.Unmarshal([]byte(t.Content), config); err != nil {
		e := fmt.Errorf("Unable to unmarshal configuration (%s): %s", err, t.Content)
		return nil, e
	}

	var finalLayout *Layout
	needResolve := map[string]*LayoutResource{}

	// Start things off by attempting to resolve the templates in a first pass.
	newImp, err := e.typeResolver.ResolveTypes(config, t.Imports)
	if err != nil {
		e := fmt.Errorf("type resolution failed: %s", err)
		return nil, expanderError(&t, e)
	}

	t.Imports = append(t.Imports, newImp...)

	for {
		// Now expand with everything imported.
		result, err := e.expandTemplate(&t)
		if err != nil {
			e := fmt.Errorf("template expansion: %s", err)
			return nil, expanderError(&t, e)
		}

		// Once we set this layout, we're operating on the "needResolve" *LayoutResources,
		// which are pointers into the original layout structure. After each expansion we
		// lose the templates in the previous expansion, so we have to keep the first one
		// around and keep appending to the pointers in it as we get more layers of expansion.
		if finalLayout == nil {
			finalLayout = result.Layout
		}
		needResolve = walkLayout(result.Layout, needResolve)

		newImp, err = e.typeResolver.ResolveTypes(result.Config, nil)
		if err != nil {
			e := fmt.Errorf("type resolution failed: %s", err)
			return nil, expanderError(&t, e)
		}

		// If the new imports contain nothing, we are done. Everything is fully expanded.
		if len(newImp) == 0 {
			result.Layout = finalLayout
			return result, nil
		}

		t.Imports = append(t.Imports, newImp...)
		var content []byte
		content, err = yaml.Marshal(result.Config)
		t.Content = string(content)
		if err != nil {
			e := fmt.Errorf("Unable to unmarshal response from expander (%s): %s",
				err, result.Config)
			return nil, expanderError(&t, e)
		}

	}
}

func (e *expander) expandTemplate(t *Template) (*ExpandedTemplate, error) {
	j, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	response, err := http.Post(e.getBaseURL(), "application/json", ioutil.NopCloser(bytes.NewReader(j)))
	if err != nil {
		e := fmt.Errorf("http POST failed: %s", err)
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

	er := &ExpansionResponse{}
	if err := json.Unmarshal(body, er); err != nil {
		e := fmt.Errorf("cannot unmarshal response body (%s):%s", err, body)
		return nil, e
	}

	template, err := er.Unmarshal()
	if err != nil {
		e := fmt.Errorf("cannot unmarshal response yaml (%s):%v", err, er)
		return nil, e
	}

	return template, nil
}

// ExpansionResponse describes the results of marshaling an ExpandedTemplate.
type ExpansionResponse struct {
	Config string `json:"config"`
	Layout string `json:"layout"`
}

// Unmarshal creates and returns an ExpandedTemplate from an ExpansionResponse.
func (er *ExpansionResponse) Unmarshal() (*ExpandedTemplate, error) {
	template := &ExpandedTemplate{}
	if err := yaml.Unmarshal([]byte(er.Config), &template.Config); err != nil {
		return nil, fmt.Errorf("cannot unmarshal config (%s):\n%s", err, er.Config)
	}

	if err := yaml.Unmarshal([]byte(er.Layout), &template.Layout); err != nil {
		return nil, fmt.Errorf("cannot unmarshal layout (%s):\n%s", err, er.Layout)
	}

	return template, nil
}
