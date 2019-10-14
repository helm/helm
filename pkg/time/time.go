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

// Package time contains a wrapper for time.Time in the standard library and
// associated methods. This package mainly exists to workaround an issue in Go
// where the serializer doesn't omit an empty value for time:
// https://github.com/golang/go/issues/11939. As such, this can be removed if a
// proposal is ever accepted for Go
package time

import (
	"bytes"
	"time"
)

// emptyString contains an empty JSON string value to be used as output
var emptyString = `""`

// Time is a convenience wrapper around stdlib time, but with different
// marshalling and unmarshaling for zero values
type Time struct {
	time.Time
}

// Now returns the current time. It is a convenience wrapper around time.Now()
func Now() Time {
	return Time{time.Now()}
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte(emptyString), nil
	}

	return t.Time.MarshalJSON()
}

func (t *Time) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	// If it is empty, we don't have to set anything since time.Time is not a
	// pointer and will be set to the zero value
	if bytes.Equal([]byte(emptyString), b) {
		return nil
	}

	return t.Time.UnmarshalJSON(b)
}
