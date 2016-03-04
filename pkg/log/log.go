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

/*
Package log provides simple convenience wrappers for logging.

Following convention, this provides functions for logging warnings, errors, information
and debugging.
*/
package log

import (
	"log"
	"os"
)

// Receiver can receive log messages from this package.
type Receiver interface {
	Printf(format string, v ...interface{})
}

// Logger is the destination for this package.
//
// The logger that this prints to.
var Logger Receiver = log.New(os.Stderr, "", log.LstdFlags)

// IsDebugging controls debugging output.
//
// If this is true, debugging messages will be printed. Expensive debugging
// operations can be wrapped in `if log.IsDebugging {}`.
var IsDebugging = false

// Err prints an error of severity ERROR to the log.
func Err(msg string, v ...interface{}) {
	Logger.Printf("[ERROR] "+msg+"\n", v...)
}

// Warn prints an error severity WARN to the log.
func Warn(msg string, v ...interface{}) {
	Logger.Printf("[WARN] "+msg+"\n", v...)
}

// Info prints an error of severity INFO to the log.
func Info(msg string, v ...interface{}) {
	Logger.Printf("[INFO] "+msg+"\n", v...)
}

// Debug prints an error severity DEBUG to the log.
//
// Debug will only print if IsDebugging is true.
func Debug(msg string, v ...interface{}) {
	if IsDebugging {
		Logger.Printf("[DEBUG] "+msg+"\n", v...)
	}
}
