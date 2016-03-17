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
	"fmt"
	"log"
	"net/http"
)

const (
	// LogAccess is for logging access messages. Form: Access r.Method, r.URL
	LogAccess = "Access: %s %s"
	// LogNotFound is for logging 404 errors. Form: Not Found r.Method, r.URL
	LogNotFound = "Not Found: %s %s"
	// LogFatal is for logging 500 errors. Form: Internal Server Error r.Method r.URL message
	LogFatal = "Internal Server Error: %s %s %s"
	// LogBadRequest logs 400 errors.
	LogBadRequest = "Bad Request: %s %s %s"
)

// Error represents an HTTP error that can be converted to structured types.
//
// For example, and error can be serialized to JSON or YAML. Likewise, the
// string marshal can convert it to a string.
type Error struct {
	Status string `json:"status"`
	Msg    string `json:"message, omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Status, e.Msg)
}

// NotFound writes a 404 error to the client and logs an error.
func NotFound(w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprintf(LogNotFound, r.Method, r.URL)
	log.Println(msg)
	writeErr(w, r, msg, http.StatusNotFound)
}

// RequestEntityTooLarge writes a 413 to the client and logs an error.
func RequestEntityTooLarge(w http.ResponseWriter, r *http.Request, msg string) {
	log.Println(msg)
	writeErr(w, r, msg, http.StatusRequestEntityTooLarge)
}

// BadRequest writes an HTTP 400.
func BadRequest(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf(LogBadRequest, r.Method, r.URL, err)
	writeErr(w, r, err.Error(), http.StatusBadRequest)
}

// writeErr formats and writes the error using the default encoder.
func writeErr(w http.ResponseWriter, r *http.Request, msg string, status int) {
	DefaultEncoder.Encode(w, r, &Error{Status: http.StatusText(status), Msg: msg}, status)
}

// Fatal writes a 500 response to the client and logs the message.
//
// Additional arguments are past into the the formatter as params to msg.
func Fatal(w http.ResponseWriter, r *http.Request, msg string, v ...interface{}) {
	m := fmt.Sprintf(msg, v...)
	log.Printf(LogFatal, r.Method, r.URL, m)
	writeErr(w, r, m, http.StatusInternalServerError)
}
