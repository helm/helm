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

package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// Format is a type for capturing supported output formats
type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	YAML  Format = "yaml"
)

// Formats returns a list of the string representation of the supported formats
func Formats() []string {
	return []string{Table.String(), JSON.String(), YAML.String()}
}

// FormatsWithDesc returns a list of the string representation of the supported formats
// including a description
func FormatsWithDesc() map[string]string {
	return map[string]string{
		Table.String(): "Output result in human-readable format",
		JSON.String():  "Output result in JSON format",
		YAML.String():  "Output result in YAML format",
	}
}

// ErrInvalidFormatType is returned when an unsupported format type is used
var ErrInvalidFormatType = fmt.Errorf("invalid format type")

// String returns the string representation of the Format
func (o Format) String() string {
	return string(o)
}

// Write the output in the given format to the io.Writer. Unsupported formats
// will return an error
func (o Format) Write(out io.Writer, w Writer) error {
	switch o {
	case Table:
		return w.WriteTable(out)
	case JSON:
		return w.WriteJSON(out)
	case YAML:
		return w.WriteYAML(out)
	}
	return ErrInvalidFormatType
}

// ParseFormat takes a raw string and returns the matching Format.
// If the format does not exists, ErrInvalidFormatType is returned
func ParseFormat(s string) (out Format, err error) {
	switch s {
	case Table.String():
		out, err = Table, nil
	case JSON.String():
		out, err = JSON, nil
	case YAML.String():
		out, err = YAML, nil
	default:
		out, err = "", ErrInvalidFormatType
	}
	return
}

// Writer is an interface that any type can implement to write supported formats
type Writer interface {
	// WriteTable will write tabular output into the given io.Writer, returning
	// an error if any occur
	WriteTable(out io.Writer) error
	// WriteJSON will write JSON formatted output into the given io.Writer,
	// returning an error if any occur
	WriteJSON(out io.Writer) error
	// WriteYAML will write YAML formatted output into the given io.Writer,
	// returning an error if any occur
	WriteYAML(out io.Writer) error
}

// EncodeJSON is a helper function to decorate any error message with a bit more
// context and avoid writing the same code over and over for printers.
func EncodeJSON(out io.Writer, obj interface{}) error {
	enc := json.NewEncoder(out)
	err := enc.Encode(obj)
	if err != nil {
		return errors.Wrap(err, "unable to write JSON output")
	}
	return nil
}

// EncodeYAML is a helper function to decorate any error message with a bit more
// context and avoid writing the same code over and over for printers
func EncodeYAML(out io.Writer, obj interface{}) error {
	raw, err := yaml.Marshal(obj)
	if err != nil {
		return errors.Wrap(err, "unable to write YAML output")
	}

	_, err = out.Write(raw)
	if err != nil {
		return errors.Wrap(err, "unable to write YAML output")
	}
	return nil
}

// EncodeTable is a helper function to decorate any error message with a bit
// more context and avoid writing the same code over and over for printers
func EncodeTable(out io.Writer, table *uitable.Table) error {
	raw := table.Bytes()
	raw = append(raw, []byte("\n")...)
	_, err := out.Write(raw)
	if err != nil {
		return errors.Wrap(err, "unable to write table output")
	}
	return nil
}
