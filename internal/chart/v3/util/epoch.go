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
	"fmt"
	"os"
	"strconv"
	"time"

	chart "helm.sh/helm/v4/internal/chart/v3"
)

// ParseSourceDateEpoch reads the SOURCE_DATE_EPOCH environment variable and
// returns the corresponding time. It returns the zero time when the variable
// is not set. An error is returned when the value cannot be parsed or is
// negative.
//
// SOURCE_DATE_EPOCH is a standardised environment variable for reproducible
// builds; see https://reproducible-builds.org/docs/source-date-epoch/
func ParseSourceDateEpoch() (time.Time, error) {
	v, ok := os.LookupEnv("SOURCE_DATE_EPOCH")
	if !ok || v == "" {
		return time.Time{}, nil
	}
	epoch, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid SOURCE_DATE_EPOCH %q: %w", v, err)
	}
	if epoch < 0 {
		return time.Time{}, fmt.Errorf("invalid SOURCE_DATE_EPOCH %q: negative value", v)
	}
	return time.Unix(epoch, 0), nil
}

// ApplySourceDateEpoch sets the ModTime on the chart and all of its entries
// that currently have a zero ModTime to t. It recurses into dependencies.
// When t is the zero time this is a no-op.
func ApplySourceDateEpoch(c *chart.Chart, t time.Time) {
	if t.IsZero() {
		return
	}
	if c.ModTime.IsZero() {
		c.ModTime = t
	}
	if c.Lock != nil && c.Lock.Generated.IsZero() {
		c.Lock.Generated = t
	}
	if c.Schema != nil && c.SchemaModTime.IsZero() {
		c.SchemaModTime = t
	}
	for _, f := range c.Raw {
		if f.ModTime.IsZero() {
			f.ModTime = t
		}
	}
	for _, f := range c.Templates {
		if f.ModTime.IsZero() {
			f.ModTime = t
		}
	}
	for _, f := range c.Files {
		if f.ModTime.IsZero() {
			f.ModTime = t
		}
	}
	for _, dep := range c.Dependencies() {
		ApplySourceDateEpoch(dep, t)
	}
}
