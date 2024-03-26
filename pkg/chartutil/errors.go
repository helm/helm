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
	"fmt"
)

// ErrNoTable indicates that a chart does not have a matching table.
type ErrNoTable struct {
	Key string
}

func (e ErrNoTable) Error() string { return fmt.Sprintf("%q is not a table", e.Key) }

// ErrNoValue indicates that Values does not contain a key with a value
type ErrNoValue struct {
	Key string
}

func (e ErrNoValue) Error() string { return fmt.Sprintf("%q is not a value", e.Key) }

type ErrInvalidChartName struct {
	Name string
}

func (e ErrInvalidChartName) Error() string {
	return fmt.Sprintf("%q is not a valid chart name", e.Name)
}
