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
)

// NotFound writes a 404 error to the client and logs an error.
func NotFound(w http.ResponseWriter, r *http.Request) {
	log.Printf(LogNotFound, r.Method, r.URL)
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintln(w, "File Not Found")
}

// Fatal writes a 500 response to the client and logs the message.
//
// Additional arguments are past into the the formatter as params to msg.
func Fatal(w http.ResponseWriter, r *http.Request, msg string, v ...interface{}) {
	log.Printf(LogFatal, r.Method, r.URL, fmt.Sprintf(msg, v...))
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintln(w, "Internal Server Error")
}
