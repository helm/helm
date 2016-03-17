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
	"io/ioutil"
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"
)

// DefaultEncoder is an *AcceptEncoder with the default application/json encoding.
var DefaultEncoder = &AcceptEncoder{DefaultEncoding: "application/json", MaxReadLen: DefaultMaxReadLen}

// DefaultMaxReadLen is the default maximum length to accept in an HTTP request body.
var DefaultMaxReadLen int64 = 1024 * 1024

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
	//
	// The integer must be a valid http.Status* status code.
	Encode(http.ResponseWriter, *http.Request, interface{}, int)

	// Decode reads and decodes a request body.
	Decode(http.ResponseWriter, *http.Request, interface{}) error
}

// Decode decodes a request body using the DefaultEncoder.
func Decode(w http.ResponseWriter, r *http.Request, v interface{}) error {
	return DefaultEncoder.Decode(w, r, v)
}

// Encode encodes a request body using the DefaultEncoder.
func Encode(w http.ResponseWriter, r *http.Request, v interface{}, statusCode int) {
	DefaultEncoder.Encode(w, r, v, statusCode)
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
	MaxReadLen      int64
}

// Encode encodeds the given interface to the first available type in the Accept header.
func (e *AcceptEncoder) Encode(w http.ResponseWriter, r *http.Request, out interface{}, statusCode int) {
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
	w.WriteHeader(statusCode)
	w.Write(data)
}

// Decode decodes the given request into the given interface.
//
// It selects the marshal based on the value of the Content-Type header. If no
// viable decoder is found, it attempts to use the DefaultEncoder.
func (e *AcceptEncoder) Decode(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if e.MaxReadLen > 0 && r.ContentLength > int64(e.MaxReadLen) {
		RequestEntityTooLarge(w, r, fmt.Sprintf("Max len is %d, submitted len is %d.", e.MaxReadLen, r.ContentLength))
	}
	data, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return err
	}

	ct := r.Header.Get("content-type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		mt = "application/x-octet-stream"
	}

	for n, fn := range decoders {
		if n == mt {
			return fn(data, v)
		}
	}

	return decoders[e.DefaultEncoding](data, v)
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

// Unmarshaler unmarshals []byte to an interface{}.
type Unmarshaler func([]byte, interface{}) error

var encoders = map[string]Marshaler{
	"application/json":   json.Marshal,
	"text/yaml":          yaml.Marshal,
	"application/x-yaml": yaml.Marshal,
	"text/plain":         textMarshal,
}

var decoders = map[string]Unmarshaler{
	"application/json":   json.Unmarshal,
	"text/yaml":          yaml.Unmarshal,
	"application/x-yaml": yaml.Unmarshal,
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
