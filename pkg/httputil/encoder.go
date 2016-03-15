/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"
)

// Encoder takes input and translate it to an expected encoded output.
//
// Implementations of encoders may use details of the HTTP request and response
// to correctly encode an object for return to the client.
//
// Encoders are expected to produce output, even if that output is an error
// message.
type Encoder interface {
	// Encode encoders a given response
	//
	// When an encoder fails, it logs any necessary data and then responds to
	// the client.
	Encode(http.ResponseWriter, *http.Request, interface{})
}

// AcceptEncoder uses the accept headers on a request to determine the response type.
//
// It supports the following encodings:
//	- application/json: passed to encoding/json.Marshal
//	- text/yaml: passed to gopkg.in/yaml.v2.Marshal
//	- text/plain: passed to fmt.Sprintf("%V")
//
// It treats `application/x-yaml` as `text/yaml`.
type AcceptEncoder struct {
	DefaultEncoding string
}

// Encode encodeds the given interface to the first available type in the Accept header.
func (e *AcceptEncoder) Encode(w http.ResponseWriter, r *http.Request, out interface{}) {
	a := r.Header.Get("accept")
	fn := encoders[e.DefaultEncoding]
	mt := e.DefaultEncoding
	if a != "" {
		mt, fn = e.parseAccept(a)
	}

	data, err := fn(out)
	if err != nil {
		Fatal(w, r, "Could not marshal data: %s", err)
		return
	}
	w.Header().Add("content-type", mt)
	w.Write(data)
}

// parseAccept parses the value of an Accept: header and returns the best match.
//
// This returns the matched MIME type and the Marshal function.
func (e *AcceptEncoder) parseAccept(h string) (string, Marshaler) {

	keys := strings.Split(h, ",")
	for _, k := range keys {
		mt, _, err := mime.ParseMediaType(k)
		if err != nil {
			continue
		}
		if enc, ok := encoders[mt]; ok {
			return mt, enc
		}
	}
	return e.DefaultEncoding, encoders[e.DefaultEncoding]
}

// Marshaler marshals an interface{} into a []byte.
type Marshaler func(interface{}) ([]byte, error)

var encoders = map[string]Marshaler{
	"application/json":   json.Marshal,
	"text/yaml":          yaml.Marshal,
	"application/x-yaml": yaml.Marshal,
	"text/plain":         textMarshal,
}

// ErrUnsupportedKind indicates that the marshal cannot marshal a particular Go Kind (e.g. struct or chan).
var ErrUnsupportedKind = errors.New("unsupported kind")

// textMarshal marshals v into a text representation ONLY IN NARROW CASES.
//
//	An error will have its Error() method called.
//	A fmt.Stringer will have its String() method called.
//	Scalar types will be marshaled with fmt.Sprintf("%v").
//
// This will only marshal scalar types for securoty reasons (namely, we don't
// want the possibility of forcing exposure of non-exported data or ptr
// addresses, etc.)
func textMarshal(v interface{}) ([]byte, error) {
	switch s := v.(type) {
	case error:
		return []byte(s.Error()), nil
	case fmt.Stringer:
		return []byte(s.String()), nil
	}

	// Error on kinds we don't support.
	val := reflect.Indirect(reflect.ValueOf(v))
	switch val.Kind() {
	case reflect.Invalid, reflect.Array, reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Ptr, reflect.Slice, reflect.Struct, reflect.UnsafePointer:
		return []byte{}, ErrUnsupportedKind
	}
	return []byte(fmt.Sprintf("%v", v)), nil
}
