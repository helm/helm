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

package engine

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// funcMap returns a mapping of all of the functions that Engine has.
//
// Because some functions are late-bound (e.g. contain context-sensitive
// data), the functions may not all perform identically outside of an Engine
// as they will inside of an Engine.
//
// Known late-bound functions:
//
//	- "include"
//	- "tpl"
//
// These are late-bound in Engine.Render().  The
// version included in the FuncMap is a placeholder.
//
func funcMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	// Add some extra functionality
	extra := template.FuncMap{
		"toToml":   toTOML,
		"toYaml":   toYAML,
		"fromYaml": fromYAML,
		"toJson":   toJSON,
		"fromJson": fromJSON,
		"required": required,

		// This is a placeholder for the "include" function, which is
		// late-bound to a template. By declaring it here, we preserve the
		// integrity of the linter.
		"include": func(string, interface{}) string { return "not implemented" },
		"tpl":     func(string, interface{}) interface{} { return "not implemented" },
	}

	for k, v := range extra {
		f[k] = v
	}

	return f
}

func required(warn string, val interface{}) (interface{}, error) {
	if val == nil {
		return val, errors.Errorf(warn)
	} else if _, ok := val.(string); ok {
		if val == "" {
			return val, errors.Errorf(warn)
		}
	}
	return val, nil
}

// toYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

// fromYAML converts a YAML document into a map[string]interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// toTOML takes an interface, marshals it to toml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toTOML(v interface{}) string {
	b := bytes.NewBuffer(nil)
	e := toml.NewEncoder(b)
	err := e.Encode(v)
	if err != nil {
		return err.Error()
	}
	return b.String()
}

// toJSON takes an interface, marshals it to json, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return string(data)
}

// fromJSON converts a JSON document into a map[string]interface{}.
//
// This is not a general-purpose JSON parser, and will not parse all valid
// JSON documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromJSON(str string) map[string]interface{} {
	m := make(map[string]interface{})

	if err := json.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}
