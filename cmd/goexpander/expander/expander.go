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

package expander

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/ghodss/yaml"
	"github.com/kubernetes/helm/pkg/expansion"
)

// parseYAMLStream takes an encoded YAML stream and turns it into a slice of JSON-marshalable
// objects, one for each document in the stream.
func parseYAMLStream(in io.Reader) ([]interface{}, error) {
	// Use candiedyaml because it's the only one that supports streams.
	decoder := candiedyaml.NewDecoder(in)
	var document interface{}
	stream := []interface{}{}
	for {
		err := decoder.Decode(&document)
		if err != nil {
			if strings.Contains(err.Error(), "Expected document start at line") {
				return stream, nil
			}
			return nil, err
		}
		// Now it's held in document but we have to do a bit of a dance to get it in a form that can
		// be marshaled as JSON for our API response.  The fundamental problem is that YAML is a
		// superset of JSON in that it can represent non-string keys, and full IEEE floating point
		// values (NaN etc).  JSON only allows string keys and its definition of a number is based
		// around a sequence of digits.

		// Kubernetes does not make use of these features, as it uses YAML as just "pretty JSON".
		// Consequently this does not affect Helm either.  However, both candiedyaml and go-yaml
		// return types that are too wide for JSON marshalling (i.e. map[interface{}]interface{}
		// instead of map[string]interface{}), so we have to do some explicit conversion.  Luckily,
		// ghodss/yaml has code to help with this, since decoding from YAML to JSON-marshalable
		// values is exactly the problem that it was designed to solve.

		// 1) Marshal it back to YAML string.
		yamlBytes, err := candiedyaml.Marshal(document)
		if err != nil {
			return nil, err
		}

		// 2) Use ghodss/yaml to unmarshal that string into JSON-compatible data structures.
		var jsonObj interface{}
		if err := yaml.Unmarshal(yamlBytes, &jsonObj); err != nil {
			return nil, err
		}

		// Now it's suitable for embedding in an API response.
		stream = append(stream, jsonObj)
	}
}

type expander struct {
}

// NewExpander returns an Go Templating expander.
func NewExpander() expansion.Expander {
	return &expander{}
}

// ExpandChart resolves the given files to a sequence of JSON-marshalable values.
func (e *expander) ExpandChart(request *expansion.ServiceRequest) (*expansion.ServiceResponse, error) {

	err := expansion.ValidateRequest(request)
	if err != nil {
		return nil, err
	}

	// TODO(dcunnin): Validate via JSONschema.

	chartInv := request.ChartInvocation
	chartMembers := request.Chart.Members

	resources := []interface{}{}
	for _, file := range chartMembers {
		name := file.Path
		content := file.Content
		tmpl := template.New(name).Funcs(sprig.HermeticTxtFuncMap())

		for _, otherFile := range chartMembers {
			otherName := otherFile.Path
			otherContent := otherFile.Content
			if name == otherName {
				continue
			}
			_, err := tmpl.Parse(string(otherContent))
			if err != nil {
				return nil, err
			}
		}

		// Have to put something in that resolves non-empty or Go templates get confused.
		_, err := tmpl.Parse("# Content begins now")
		if err != nil {
			return nil, err
		}

		tmpl, err = tmpl.Parse(string(content))
		if err != nil {
			return nil, err
		}

		generated := bytes.NewBuffer(nil)
		if err := tmpl.ExecuteTemplate(generated, name, chartInv.Properties); err != nil {
			return nil, err
		}

		stream, err := parseYAMLStream(generated)
		if err != nil {
			return nil, fmt.Errorf("%s\nContent:\n%s", err.Error(), generated)
		}

		for _, doc := range stream {
			resources = append(resources, doc)
		}
	}

	return &expansion.ServiceResponse{Resources: resources}, nil
}
