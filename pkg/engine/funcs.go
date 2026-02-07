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
	"fmt"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

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

		// Duration helpers
		"mustToDuration":     mustToDuration,
		"toSeconds":          toSeconds,
		"toMilliseconds":     toMilliseconds,
		"toMicroseconds":     toMicroseconds,
		"toNanoseconds":      toNanoseconds,
		"toMinutes":          toMinutes,
		"toHours":            toHours,
		"toDays":             toDays,
		"toWeeks":            toWeeks,
		"roundToDuration":    roundToDuration,
		"truncateToDuration": truncateToDuration,

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

// -----------------------------------------------------------------------------
// Duration helpers (numeric-only returns)
// -----------------------------------------------------------------------------

// asDuration converts common template values into a time.Duration.
//
// Supported inputs:
//   - time.Duration
//   - string duration values parsed by time.ParseDuration (e.g. "1h2m3s")
//   - numeric strings treated as seconds (e.g. "2.5")
//   - ints and uints treated as seconds
//   - floats treated as seconds
func asDuration(v interface{}) (time.Duration, error) {
	switch x := v.(type) {
	case time.Duration:
		return x, nil

	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, fmt.Errorf("empty duration")
		}
		if d, err := time.ParseDuration(s); err == nil {
			return d, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return time.Duration(f * float64(time.Second)), nil
		}
		return 0, fmt.Errorf("could not parse duration %q", x)

	case nil:
		return 0, fmt.Errorf("invalid duration")
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return time.Duration(rv.Int()) * time.Second, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u := rv.Uint()
		if u > uint64(math.MaxInt64) {
			return 0, fmt.Errorf("duration seconds overflow: %d", u)
		}
		return time.Duration(int64(u)) * time.Second, nil
	case reflect.Float32, reflect.Float64:
		return time.Duration(rv.Float() * float64(time.Second)), nil
	default:
		return 0, fmt.Errorf("unsupported duration type %T", v)
	}
}

// mustToDuration takes an interface, parses a duration, and returns a time.Duration.
// It will panic if there is an error.
//
// This is designed to be called from a template when need to ensure that a
// duration is valid.
func mustToDuration(v interface{}) time.Duration {
	d, err := asDuration(v)
	if err != nil {
		panic(err)
	}
	return d
}

// toSeconds converts a duration to seconds (float64).
// On error it returns 0.
func toSeconds(v interface{}) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Seconds()
}

// toMilliseconds converts a duration to milliseconds (int64).
// On error it returns 0.
func toMilliseconds(v interface{}) int64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Milliseconds()
}

// toMicroseconds converts a duration to microseconds (int64).
func toMicroseconds(v interface{}) int64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Microseconds()
}

// toNanoseconds converts a duration to nanoseconds (int64).
// On error it returns 0.
func toNanoseconds(v interface{}) int64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Nanoseconds()
}

// toMinutes converts a duration to minutes (float64).
func toMinutes(v interface{}) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Minutes()
}

// toHours converts a duration to hours (float64).
// On error it returns 0.
func toHours(v interface{}) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Hours()
}

// toDays converts a duration to days (float64). (Not in Go's stdlib; handy in templates.)
// On error it returns 0.
func toDays(v interface{}) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Hours() / 24.0
}

// toWeeks converts a duration to weeks (float64). (Not in Go's stdlib; handy in templates.)
// On error it returns 0.
func toWeeks(v interface{}) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Hours() / 24.0 / 7.0
}

// roundToDuration rounds v to the nearest multiple of m.
// Returns a time.Duration.
//
// v and m accept the same forms as asDuration (e.g. "2h13m", "30s").
// On error, it returns time.Duration(0). If m is invalid, it returns v.
func roundToDuration(v interface{}, m interface{}) time.Duration {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	mul, err := asDuration(m)
	if err != nil {
		return d
	}
	return d.Round(mul)
}

// truncateToDuration truncates v toward zero to a multiple of m.
// Returns a time.Duration.
//
// On error, it returns time.Duration(0). If m is invalid, it returns v.
func truncateToDuration(v interface{}, m interface{}) time.Duration {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	mul, err := asDuration(m)
	if err != nil {
		return d
	}
	return d.Truncate(mul)
}
