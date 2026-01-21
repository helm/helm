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
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"
	"text/template"
	"unicode/utf8"

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
		"gzip":          gzipFunc,
		"ungzip":        ungzipFunc,

		// This is a placeholder for the "include" function, which is
		// late-bound to a template. By declaring it here, we preserve the
		// integrity of the linter.
		"include":  func(string, interface{}) string { return "not implemented" },
		"tpl":      func(string, interface{}) interface{} { return "not implemented" },
		"required": func(string, interface{}) (interface{}, error) { return "not implemented", nil },
		// Provide a placeholder for the "lookup" function, which requires a kubernetes
		// connection.
		"lookup": func(string, string, string, string) (map[string]interface{}, error) {
			return map[string]interface{}{}, nil
		},
	}

	maps.Copy(f, extra)

	return f
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

// mustToYAML takes an interface, marshals it to yaml, and returns a string.
// It will panic if there is an error.
//
// This is designed to be called from a template when need to ensure that the
// output YAML is valid.
func mustToYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		panic(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func toYAMLPretty(v interface{}) string {
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
func fromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

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
func fromYAMLArray(str string) []interface{} {
	a := []interface{}{}

	if err := yaml.Unmarshal([]byte(str), &a); err != nil {
		a = []interface{}{err.Error()}
	}
	return a
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

// fromTOML converts a TOML document into a map[string]interface{}.
//
// This is not a general-purpose TOML parser, and will not parse all valid
// TOML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromTOML(str string) map[string]interface{} {
	m := make(map[string]interface{})

	if err := toml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
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

// mustToJSON takes an interface, marshals it to json, and returns a string.
// It will panic if there is an error.
//
// This is designed to be called from a template when need to ensure that the
// output JSON is valid.
func mustToJSON(v interface{}) string {
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
func fromJSON(str string) map[string]interface{} {
	m := make(map[string]interface{})

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
func fromJSONArray(str string) []interface{} {
	a := []interface{}{}

	if err := json.Unmarshal([]byte(str), &a); err != nil {
		a = []interface{}{err.Error()}
	}
	return a
}

// gzipFunc compresses a string using gzip and returns the base64 encoded result.
//
// It enforces a size limit (1MB) on the output to prevent creating objects too large for Kubernetes Secrets/ConfigMap.
//
// This is designed to be called from a template.
func gzipFunc(str string) (string, error) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write([]byte(str)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(b.Bytes())

	// Kubernetes limit for Secret/ConfigMap is 1MB.
	const maxLimit = 1048576
	if len(encoded) > maxLimit {
		return "", fmt.Errorf("gzip: output size %d exceeds limit of %d bytes", len(encoded), maxLimit)
	}

	return encoded, nil
}

// ungzipFunc decodes a base64 encoded and gzip-compressed string.
//
// It enforces a size limit (1MB) on the input and output to prevent abuse.
//
// This is designed to be called from a template.
func ungzipFunc(str string) (string, error) {
	// Kubernetes limit for Secret/ConfigMap is 1MB.
	const maxLimit = 1048576

	if len(str) > maxLimit {
		return "", fmt.Errorf("ungzip: input size %d exceeds limit of %d bytes", len(str), maxLimit)
	}

	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", fmt.Errorf("ungzip: base64 decode failed: %w", err)
	}

	r, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return "", err
	}
	defer r.Close()

	// Enforce a size limit on the decompressed content.
	limitR := io.LimitReader(r, maxLimit+1)

	b, err := io.ReadAll(limitR)
	if err != nil {
		return "", err
	}

	if len(b) > maxLimit {
		return "", fmt.Errorf("ungzip: decompressed content exceeds size limit of %d bytes", maxLimit)
	}

	// Ensure the content is valid text (UTF-8) to prevent binary obfuscation.
	if !utf8.Valid(b) {
		return "", fmt.Errorf("ungzip: content is not valid UTF-8 text")
	}

	return string(b), nil
}
