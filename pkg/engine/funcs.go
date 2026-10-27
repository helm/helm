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
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
	goYaml "sigs.k8s.io/yaml/goyaml.v3"

	"helm.sh/helm/v4/pkg/chart/loader/archive"
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
		"compress":      compress,
		"decompress":    decompress,

		// Duration helpers
		"mustToDuration":       mustToDuration,
		"durationSeconds":      durationSeconds,
		"durationMilliseconds": durationMilliseconds,
		"durationMicroseconds": durationMicroseconds,
		"durationNanoseconds":  durationNanoseconds,
		"durationMinutes":      durationMinutes,
		"durationHours":        durationHours,
		"durationDays":         durationDays,
		"durationWeeks":        durationWeeks,
		"durationRoundTo":      durationRoundTo,
		"durationTruncateTo":   durationTruncateTo,

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
	err := e.Encode(v)
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
	err := e.Encode(v)
	if err != nil {
		panic(err)
	}
	return b.String()
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

// -----------------------------------------------------------------------------
// Duration helpers (numeric and time.Duration returns)
// -----------------------------------------------------------------------------

const (
	maxDurationSeconds      = int64(math.MaxInt64 / int64(time.Second))
	minDurationSeconds      = int64(math.MinInt64 / int64(time.Second))
	maxDurationSecondsFloat = float64(math.MaxInt64) / float64(time.Second)
	minDurationSecondsFloat = float64(math.MinInt64) / float64(time.Second)
)

func durationFromSecondsInt(seconds int64) (time.Duration, error) {
	if seconds > maxDurationSeconds || seconds < minDurationSeconds {
		return 0, fmt.Errorf("duration seconds overflow: %d", seconds)
	}
	return time.Duration(seconds) * time.Second, nil
}

func durationFromSecondsUint(seconds uint64) (time.Duration, error) {
	if seconds > uint64(maxDurationSeconds) {
		return 0, fmt.Errorf("duration seconds overflow: %d", seconds)
	}
	return time.Duration(int64(seconds)) * time.Second, nil
}

func durationFromSecondsFloat(seconds float64) (time.Duration, error) {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, fmt.Errorf("invalid duration seconds: %v", seconds)
	}
	if seconds > maxDurationSecondsFloat || seconds < minDurationSecondsFloat {
		return 0, fmt.Errorf("duration seconds overflow: %v", seconds)
	}
	nanos := seconds * float64(time.Second)
	if nanos > float64(math.MaxInt64) || nanos < float64(math.MinInt64) {
		return 0, fmt.Errorf("duration nanoseconds overflow: %v", nanos)
	}
	return time.Duration(nanos), nil
}

// asDuration converts common template values into a time.Duration.
//
// Supported inputs:
//   - time.Duration
//   - string duration values parsed by time.ParseDuration (e.g. "1h2m3s")
//   - numeric strings treated as seconds (e.g. "2.5")
//   - ints and uints treated as seconds
//   - floats treated as seconds
func asDuration(v any) (time.Duration, error) {
	switch x := v.(type) {
	case time.Duration:
		return x, nil

	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, errors.New("empty duration")
		}
		if d, err := time.ParseDuration(s); err == nil {
			return d, nil
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return durationFromSecondsFloat(f)
		}
		return 0, fmt.Errorf("could not parse duration %q", x)

	case nil:
		return 0, errors.New("invalid duration")
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return durationFromSecondsInt(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return durationFromSecondsUint(rv.Uint())
	case reflect.Float32, reflect.Float64:
		return durationFromSecondsFloat(rv.Float())
	default:
		return 0, fmt.Errorf("unsupported duration type %T", v)
	}
}

// mustToDuration takes anything and attempts to parse as a duration returning a time.Duration.
//
// This is designed to be called from a template when need to ensure that a
// duration is valid.
func mustToDuration(v any) time.Duration {
	d, err := asDuration(v)
	if err != nil {
		panic(err)
	}
	return d
}

// durationSeconds converts a duration to seconds (float64).
// On error it returns 0.
func durationSeconds(v any) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Seconds()
}

// durationMilliseconds converts a duration to milliseconds (int64).
// On error it returns 0.
func durationMilliseconds(v any) int64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Milliseconds()
}

// durationMicroseconds converts a duration to microseconds (int64).
// On error it returns 0.
func durationMicroseconds(v any) int64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Microseconds()
}

// durationNanoseconds converts a duration to nanoseconds (int64).
// On error it returns 0.
func durationNanoseconds(v any) int64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Nanoseconds()
}

// durationMinutes converts a duration to minutes (float64).
// On error it returns 0.
func durationMinutes(v any) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Minutes()
}

// durationHours converts a duration to hours (float64).
// On error it returns 0.
func durationHours(v any) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Hours()
}

// durationDays converts a duration to days (float64). (Not in Go's stdlib; handy in templates.)
// On error it returns 0.
func durationDays(v any) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Hours() / 24.0
}

// durationWeeks converts a duration to weeks (float64). (Not in Go's stdlib; handy in templates.)
// On error it returns 0.
func durationWeeks(v any) float64 {
	d, err := asDuration(v)
	if err != nil {
		return 0
	}
	return d.Hours() / 24.0 / 7.0
}

// durationRoundTo rounds v to the nearest multiple of m.
// Returns a time.Duration.
//
// v and m accept the same forms as asDuration (e.g. "2h13m", "30s").
// On error, it returns time.Duration(0). If m is invalid, it returns v.
func durationRoundTo(v, m any) time.Duration {
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

// durationTruncateTo truncates v toward zero to a multiple of m.
// Returns a time.Duration.
//
// On error, it returns time.Duration(0). If m is invalid, it returns v.
func durationTruncateTo(v, m any) time.Duration {
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

// -----------------------------------------------------------------------------
// Compression helpers
// -----------------------------------------------------------------------------

// compressionSizeLimit is the Kubernetes limit for Secret/ConfigMap (1MB). It
// applies to compressed data: the output of compress and the input of decompress.
const compressionSizeLimit = 1048576

// compressionAlgorithms maps the algorithm names accepted by compress and
// decompress to their implementations. The size limits and UTF-8 validation
// in compress and decompress apply to every algorithm.
var compressionAlgorithms = map[string]struct {
	newWriter func(io.Writer) io.WriteCloser
	newReader func(io.Reader) (io.ReadCloser, error)
}{
	"gzip": {
		newWriter: func(w io.Writer) io.WriteCloser { return gzip.NewWriter(w) },
		newReader: func(r io.Reader) (io.ReadCloser, error) { return gzip.NewReader(r) },
	},
}

// compress compresses a string using the given algorithm and returns the base64
// encoded result. "gzip" is currently the only supported algorithm.
//
// It enforces a size limit (1MB) on the output to prevent creating objects too large for Kubernetes Secrets/ConfigMap.
//
// This is designed to be called from a template, e.g. {{ .Files.Get "dashboard.json" | compress "gzip" }}.
func compress(algorithm, str string) (string, error) {
	alg, ok := compressionAlgorithms[algorithm]
	if !ok {
		return "", fmt.Errorf("compress: unsupported algorithm %q", algorithm)
	}

	if !utf8.ValidString(str) {
		return "", errors.New("compress: content is not valid UTF-8 text")
	}

	var b bytes.Buffer
	w := alg.newWriter(&b)
	if _, err := w.Write([]byte(str)); err != nil {
		return "", fmt.Errorf("compress: write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("compress: close failed: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(b.Bytes())
	if len(encoded) > compressionSizeLimit {
		return "", fmt.Errorf("compress: output size %d exceeds limit of %d bytes", len(encoded), compressionSizeLimit)
	}

	return encoded, nil
}

// decompress decodes a base64 encoded string and decompresses it using the
// given algorithm. "gzip" is currently the only supported algorithm.
//
// It enforces a size limit (1MB) on the input. The decompressed output may
// exceed 1MB, since compressing such data to fit in a Secret/ConfigMap is the
// main use case, but is bounded by archive.MaxDecompressedChartSize to
// prevent decompression bombs.
//
// This is designed to be called from a template, e.g. {{ .Values.dashboard | decompress "gzip" }}.
func decompress(algorithm, str string) (string, error) {
	alg, ok := compressionAlgorithms[algorithm]
	if !ok {
		return "", fmt.Errorf("decompress: unsupported algorithm %q", algorithm)
	}

	if len(str) > compressionSizeLimit {
		return "", fmt.Errorf("decompress: input size %d exceeds limit of %d bytes", len(str), compressionSizeLimit)
	}

	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", fmt.Errorf("decompress: base64 decode failed: %w", err)
	}

	r, err := alg.newReader(bytes.NewReader(decoded))
	if err != nil {
		return "", fmt.Errorf("decompress: %s reader failed: %w", algorithm, err)
	}
	defer r.Close()

	// Enforce a size limit on the decompressed content.
	maxSize := archive.MaxDecompressedChartSize
	limitR := io.LimitReader(r, maxSize+1)

	b, err := io.ReadAll(limitR)
	if err != nil {
		return "", fmt.Errorf("decompress: read failed: %w", err)
	}

	if int64(len(b)) > maxSize {
		return "", fmt.Errorf("decompress: decompressed content exceeds size limit of %d bytes", maxSize)
	}

	// Ensure the content is valid text (UTF-8) to prevent binary obfuscation.
	if !utf8.Valid(b) {
		return "", errors.New("decompress: content is not valid UTF-8 text")
	}

	return string(b), nil
}
