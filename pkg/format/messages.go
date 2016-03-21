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

package format

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"

	"github.com/ghodss/yaml"
)

// Stdout is the output this library will write to.
var Stdout io.Writer = os.Stdout

// Stderr is the error output this library will write to.
var Stderr io.Writer = os.Stderr

// This is all just placeholder.

// Err prints an error message to Stderr.
func Err(message interface{}, v ...interface{}) {
	var msg string
	val := reflect.Indirect(reflect.ValueOf(message))
	if val.Kind() == reflect.String {
		msg = message.(string)
	} else if z, ok := message.(fmt.Stringer); ok {
		msg = z.String()
	} else if z, ok := message.(error); ok {
		msg = z.Error()
	}

	msg = "[ERROR] " + msg + "\n"
	fmt.Fprintf(Stderr, msg, v...)
}

// Info prints an informational message to Stdout.
func Info(msg string, v ...interface{}) {
	msg = "[INFO] " + msg + "\n"
	fmt.Fprintf(Stdout, msg, v...)
}

// Msg prints a raw message to Stdout.
func Msg(msg string, v ...interface{}) {
	fmt.Fprintf(Stdout, msg, v...)
}

// Success is an achievement marked by pretty output.
func Success(msg string, v ...interface{}) {
	msg = "[Success] " + msg + "\n"
	fmt.Fprintf(Stdout, msg, v...)
}

// Warning emits a warning message.
func Warning(msg string, v ...interface{}) {
	msg = "[Warning] " + msg + "\n"
	fmt.Fprintf(Stdout, msg, v...)
}

// List prints a list of strings to Stdout.
//
// This sorts lexicographically.
func List(list []string) {
	sort.Strings(list)
	// Buffer and then flush all at once to avoid concurrency-based interleaving.
	var b bytes.Buffer
	for _, v := range list {
		if v == "" {
			v = "[empty]"
		}
		fmt.Fprintf(&b, "%s\n", v)
	}
	Stdout.Write(b.Bytes())
}

// YAML prints an object in YAML format.
func YAML(v interface{}) error {
	y, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("Failed to serialize to yaml: %s", v.(string))
	}

	Msg(string(y))
	return nil
}
