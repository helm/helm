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

package kube

// Logger defines a minimal logging interface compatible with slog.Logger
type Logger interface {
	Debug(msg string, args ...any)
}

// NopLogger is a logger that does nothing
type NopLogger struct{}

// Debug implements the Logger interface
func (n NopLogger) Debug(msg string, args ...any) {}

var nopLogger = NopLogger{}
