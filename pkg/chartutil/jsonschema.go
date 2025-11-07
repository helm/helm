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

package chartutil

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"net/http"

	"helm.sh/helm/v3/internal/version"
	"helm.sh/helm/v3/pkg/chart"
)

// HTTPURLLoader implements a loader for HTTP/HTTPS URLs
type HTTPURLLoader http.Client

func (l *HTTPURLLoader) Load(urlStr string) (any, error) {
	client := (*http.Client)(l)

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request for %s: %w", urlStr, err)
	}
	req.Header.Set("User-Agent", version.GetUserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed for %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request to %s returned status %d (%s)", urlStr, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return jsonschema.UnmarshalJSON(resp.Body)
}

// newHTTPURLLoader creates a HTTP URL loader with proxy support.
func newHTTPURLLoader() *HTTPURLLoader {
	httpLoader := HTTPURLLoader(http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{},
		},
	})
	return &httpLoader
}

// ValidateAgainstSchema checks that values does not violate the structure laid out in schema
func ValidateAgainstSchema(chrt *chart.Chart, values map[string]interface{}) error {
	var sb strings.Builder
	if chrt.Schema != nil {

		err := ValidateAgainstSingleSchema(values, chrt.Schema)
		if err != nil {
			sb.WriteString(fmt.Sprintf("%s:\n", chrt.Name()))
			sb.WriteString(err.Error())
		}
	}

	for _, subchart := range chrt.Dependencies() {
		raw, exists := values[subchart.Name()]
		if !exists || raw == nil {
			// No values provided for this subchart; nothing to validate
			continue
		}

		subchartValues, ok := raw.(map[string]any)
		if !ok {
			sb.WriteString(fmt.Sprintf(
				"%s:\ninvalid type for values: expected object (map), got %T\n",
				subchart.Name(), raw,
			))
			continue
		}
		if err := ValidateAgainstSchema(subchart, subchartValues); err != nil {
			sb.WriteString(err.Error())
		}
	}

	if sb.Len() > 0 {
		return errors.New(sb.String())
	}

	return nil
}

// ValidateAgainstSingleSchema checks that values does not violate the structure laid out in this schema
func ValidateAgainstSingleSchema(values Values, schemaJSON []byte) (reterr error) {
	defer func() {
		if r := recover(); r != nil {
			reterr = fmt.Errorf("unable to validate schema: %s", r)
		}
	}()

	// This unmarshal function leverages UseNumber() for number precision. The parser
	// used for values does this as well.
	schema, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return err
	}

	// Configure compiler with loaders for different URL schemes
	loader := jsonschema.SchemeURLLoader{
		"file":  jsonschema.FileLoader{},
		"http":  newHTTPURLLoader(),
		"https": newHTTPURLLoader(),
		"urn":   urnLoader{},
	}

	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(loader)
	err = compiler.AddResource("file:///values.schema.json", schema)
	if err != nil {
		return err
	}

	validator, err := compiler.Compile("file:///values.schema.json")
	if err != nil {
		return err
	}

	err = validator.Validate(values.AsMap())
	if err != nil {
		return JSONSchemaValidationError{err}
	}

	return nil
}

type JSONSchemaValidationError struct {
	embeddedErr error
}

func (e JSONSchemaValidationError) Error() string {
	errStr := e.embeddedErr.Error()

	errStr = strings.TrimPrefix(errStr, "jsonschema validation failed with 'file:///values.schema.json#'\n")

	return errStr + "\n"
}

// URNResolverFunc allows SDK to plug a URN resolver. It must return a
// schema document compatible with the validator (e.g., result of
// jsonschema.UnmarshalJSON).
type URNResolverFunc func(urn string) (any, error)

// URNResolver is the default resolver used by the URN loader. By default it
// returns a clear error.
var URNResolver URNResolverFunc = func(urn string) (any, error) {
	return nil, fmt.Errorf("URN not resolved: %s", urn)
}

// urnLoader implements resolution for the urn: scheme by delegating to
// URNResolver. If unresolved, it logs a warning and returns a permissive
// boolean-true schema to avoid hard failures (back-compat behavior).
type urnLoader struct{}

// warnedURNs ensures we log the unresolved-URN warning only once per URN.
var warnedURNs sync.Map

func (l urnLoader) Load(urlStr string) (any, error) {
	if doc, err := URNResolver(urlStr); err == nil && doc != nil {
		return doc, nil
	}
	if _, loaded := warnedURNs.LoadOrStore(urlStr, struct{}{}); !loaded {
		log.Printf("WARNING: unresolved URN reference ignored; using permissive schema: %s", urlStr)
	}
	return jsonschema.UnmarshalJSON(strings.NewReader("true"))
}
