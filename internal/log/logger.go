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

package log

// Logger defines a minimal logging interface compatible with structured logging.
// It provides methods for different log levels with structured key-value pairs.
type Logger interface {
	// Debug logs a message at debug level with structured key-value pairs.
	Debug(msg string, args ...any)

	// Warn logs a message at warning level with structured key-value pairs.
	Warn(msg string, args ...any)

	// Error logs a message at error level with structured key-value pairs.
	Error(msg string, args ...any)
}

// NopLogger is a logger implementation that discards all log messages.
type NopLogger struct{}

// Debug implements Logger.Debug by doing nothing.
func (NopLogger) Debug(_ string, args ...any) {}

// Warn implements Logger.Warn by doing nothing.
func (NopLogger) Warn(_ string, args ...any) {}

// Error implements Logger.Error by doing nothing.
func (NopLogger) Error(_ string, args ...any) {}

// DefaultLogger provides a no-op logger that discards all messages.
// It can be used as a default when no logger is provided.
var DefaultLogger Logger = NopLogger{}
