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

package util

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"helm.sh/helm/v4/internal/version"

	chart "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
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
func ValidateAgainstSchema(ch chart.Charter, values map[string]interface{}) error {
	chrt, err := chart.NewAccessor(ch)
	if err != nil {
		return err
	}
	var sb strings.Builder
	if chrt.Schema() != nil {
		slog.Debug("chart name", "chart-name", chrt.Name())
		err := ValidateAgainstSingleSchema(values, chrt.Schema())
		if err != nil {
			sb.WriteString(fmt.Sprintf("%s:\n", chrt.Name()))
			sb.WriteString(err.Error())
		}
	}
	slog.Debug("number of dependencies in the chart", "dependencies", len(chrt.Dependencies()))
	// For each dependency, recursively call this function with the coalesced values
	for _, subchart := range chrt.Dependencies() {
		sub, err := chart.NewAccessor(subchart)
		if err != nil {
			return err
		}

		raw, exists := values[sub.Name()]
		if !exists || raw == nil {
			// No values provided for this subchart; nothing to validate
			continue
		}

		subchartValues, ok := raw.(map[string]any)
		if !ok {
			sb.WriteString(fmt.Sprintf(
				"%s:\ninvalid type for values: expected object (map), got %T\n",
				sub.Name(), raw,
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
func ValidateAgainstSingleSchema(values common.Values, schemaJSON []byte) (reterr error) {
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
	slog.Debug("unmarshalled JSON schema", "schema", schemaJSON)

	// Configure compiler with loaders for different URL schemes
	loader := jsonschema.SchemeURLLoader{
		"file":  jsonschema.FileLoader{},
		"http":  newHTTPURLLoader(),
		"https": newHTTPURLLoader(),
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

// Note, JSONSchemaValidationError is used to wrap the error from the underlying
// validation package so that Helm has a clean interface and the validation package
// could be replaced without changing the Helm SDK API.

// JSONSchemaValidationError is the error returned when there is a schema validation
// error.
type JSONSchemaValidationError struct {
	embeddedErr error
}

// Error prints the error message
func (e JSONSchemaValidationError) Error() string {
	errStr := e.embeddedErr.Error()

	// This string prefixes all of our error details. Further up the stack of helm error message
	// building more detail is provided to users. This is removed.
	errStr = strings.TrimPrefix(errStr, "jsonschema validation failed with 'file:///values.schema.json#'\n")

	// The extra new line is needed for when there are sub-charts.
	return errStr + "\n"
}
