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
	"maps"
	"math"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
	goYaml "sigs.k8s.io/yaml/goyaml.v3"
)

// funcMap returns a mapping of all of the functions that Engine has.
//
// Because some functions are late-bound (e.g. contain context-sensitive
// data), the functions may not all perform identically outside of an Engine
// as they will inside of an Engine.
//
// Known late-bound functions:
//
//   - "include"
//   - "tpl"
//
// These are late-bound in Engine.Render().  The
// version included in the FuncMap is a placeholder.
func funcMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	// Add some extra functionality
	extra := template.FuncMap{
		"toToml":        toTOML,
		"mustToToml":    mustToTOML,
		"fromToml":      fromTOML,
		"toYaml":        toYAML,
		"mustToYaml":    mustToYAML,
		"toYamlPretty":  toYAMLPretty,
		"fromYaml":      fromYAML,
		"fromYamlArray": fromYAMLArray,
		"toJson":        toJSON,
		"mustToJson":    mustToJSON,
		"fromJson":      fromJSON,
		"fromJsonArray": fromJSONArray,

		// This is a placeholder for the "include" function, which is
		// late-bound to a template. By declaring it here, we preserve the
		// integrity of the linter.
		"include":  func(string, any) string { return "not implemented" },
		"tpl":      func(string, any) any { return "not implemented" },
		"required": func(string, any) (any, error) { return "not implemented", nil },
		// Provide a placeholder for the "lookup" function, which requires a kubernetes
		// connection.
		"lookup": func(string, string, string, string) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}

	maps.Copy(f, extra)

	return f
}

// toYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toYAML(v any) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

// mustToYAML takes an interface, marshals it to yaml, and returns a string.
// It will panic if there is an error.
//
// This is designed to be called from a template when need to ensure that the
// output YAML is valid.
func mustToYAML(v any) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		panic(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func toYAMLPretty(v any) string {
	var data bytes.Buffer
	encoder := goYaml.NewEncoder(&data)
	encoder.SetIndent(2)
	err := encoder.Encode(v)

	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(data.String(), "\n")
}

// fromYAML converts a YAML document into a map[string]interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromYAML(str string) map[string]any {
	m := map[string]any{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// fromYAMLArray converts a YAML array into a []interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string as
// the first and only item in the returned array.
func fromYAMLArray(str string) []any {
	a := []any{}

	if err := yaml.Unmarshal([]byte(str), &a); err != nil {
		a = []any{err.Error()}
	}
	return a
}

// toTOML takes an interface, marshals it to toml, and returns a string.
// On marshal error it returns the error string.
//
// This is designed to be called from a template. Use mustToToml if you need
// the template to fail hard on marshal errors.
func toTOML(v any) string {
	b := bytes.NewBuffer(nil)
	e := toml.NewEncoder(b)
	err := e.Encode(normalizeForTOML(v))
	if err != nil {
		return err.Error()
	}
	return b.String()
}

// mustToTOML takes an interface, marshals it to toml, and returns a string.
// It will panic if there is an error.
//
// This is designed to be called from a template when you need to ensure that the
// output TOML is valid.
func mustToTOML(v any) string {
	b := bytes.NewBuffer(nil)
	e := toml.NewEncoder(b)
	err := e.Encode(normalizeForTOML(v))
	if err != nil {
		panic(err)
	}
	return b.String()
}

// normalizeForTOML walks v and rewrites any float64 that is a whole number
// (within the int64 range) as an int64. Helm values round-trip through JSON,
// so every YAML number arrives in the template engine as a float64 regardless
// of whether the source was written as "9" or "9.0"; the BurntSushi TOML
// encoder then writes float64(9) as "9.0", which surprises users. This brings
// toToml in line with encoding/json (which already drops trailing ".0" for
// whole-number float64). Non-whole floats and values outside the int64 range
// are left untouched so that true floats still round-trip as floats.
//
// This fix is intentionally scoped to the TOML encoding path. #13533 solved
// the same issue by switching the values loader to json.Decoder.UseNumber,
// which changed every numeric value to json.Number globally and broke charts
// relying on typeOf/typeIs returning "float64" (#30880). Normalizing only
// inside toTOML preserves the in-template type of values so that regression
// cannot recur.
func normalizeForTOML(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = normalizeForTOML(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeForTOML(val)
		}
		return out
	case float64:
		// Guard against NaN/Inf and against Go's implementation-defined
		// behavior for out-of-range float-to-int conversions. 1<<63 is the
		// first positive float64 strictly greater than math.MaxInt64.
		if math.IsNaN(x) || math.IsInf(x, 0) || x != math.Trunc(x) ||
			x >= 1<<63 || x < -(1<<63) {
			return x
		}
		return int64(x)
	default:
		return v
	}
}

// fromTOML converts a TOML document into a map[string]interface{}.
//
// This is not a general-purpose TOML parser, and will not parse all valid
// TOML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromTOML(str string) map[string]any {
	m := make(map[string]any)

	if err := toml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// toJSON takes an interface, marshals it to json, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return string(data)
}

// mustToJSON takes an interface, marshals it to json, and returns a string.
// It will panic if there is an error.
//
// This is designed to be called from a template when need to ensure that the
// output JSON is valid.
func mustToJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// fromJSON converts a JSON document into a map[string]interface{}.
//
// This is not a general-purpose JSON parser, and will not parse all valid
// JSON documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromJSON(str string) map[string]any {
	m := make(map[string]any)

	if err := json.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// fromJSONArray converts a JSON array into a []interface{}.
//
// This is not a general-purpose JSON parser, and will not parse all valid
// JSON documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string as
// the first and only item in the returned array.
func fromJSONArray(str string) []any {
	a := []any{}

	if err := json.Unmarshal([]byte(str), &a); err != nil {
		a = []any{err.Error()}
	}
	return a
}
