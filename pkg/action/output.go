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

package action

import (
	"encoding/json"
	"fmt"

	"github.com/gosuri/uitable"
	"sigs.k8s.io/yaml"
)

// OutputFormat is a type for capturing supported output formats
type OutputFormat string

// TableFunc is a function that can be used to add rows to a table
type TableFunc func(tbl *uitable.Table)

const (
	Table OutputFormat = "table"
	JSON  OutputFormat = "json"
	YAML  OutputFormat = "yaml"
)

// ErrInvalidFormatType is returned when an unsupported format type is used
var ErrInvalidFormatType = fmt.Errorf("invalid format type")

// String returns the string reprsentation of the OutputFormat
func (o OutputFormat) String() string {
	return string(o)
}

// Marshal uses the specified output format to marshal out the given data. It
// does not support tabular output. For tabular output, use MarshalTable
func (o OutputFormat) Marshal(data interface{}) (byt []byte, err error) {
	switch o {
	case YAML:
		byt, err = yaml.Marshal(data)
	case JSON:
		byt, err = json.Marshal(data)
	default:
		err = ErrInvalidFormatType
	}
	return
}

// MarshalTable returns a formatted table using the given headers. Rows can be
// added to the table using the given TableFunc
func (o OutputFormat) MarshalTable(f TableFunc) ([]byte, error) {
	if o != Table {
		return nil, ErrInvalidFormatType
	}
	tbl := uitable.New()
	if f == nil {
		return []byte{}, nil
	}
	f(tbl)
	return tbl.Bytes(), nil
}

// ParseOutputFormat takes a raw string and returns the matching OutputFormat.
// If the format does not exists, ErrInvalidFormatType is returned
func ParseOutputFormat(s string) (out OutputFormat, err error) {
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
