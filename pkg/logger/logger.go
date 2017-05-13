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

package logger

// Logger provides a generic way of handling logging.
type Logger interface {
	Printf(format string, args ...interface{})
}

// Func is an adaptor to allow the use of ordinary functions as logggers.
type Func func(string, ...interface{})

// Printf implements Logger.
func (l Func) Printf(format string, args ...interface{}) {
	l(format, args...)
}

type nopLogger struct{}

// NewNopLogger creates a nopLogger.
func NewNopLogger() Logger { return nopLogger{} }

// Printf implements Logger.
func (nopLogger) Printf(format string, args ...interface{}) {}
