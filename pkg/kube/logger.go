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

import "log/slog"

// Logger defines a minimal logging interface compatible with slog.Logger
type Logger interface {
	Debug(msg string, args ...any)
	Warn(msg string, args ...any)
}

type SlogAdapter struct {
	logger *slog.Logger
}

type NopLogger struct{}

func (n NopLogger) Debug(msg string, args ...any) {}
func (n NopLogger) Warn(msg string, args ...any)  {}

var nopLogger = NopLogger{}

func (a SlogAdapter) Debug(msg string, args ...any) {
	a.logger.Debug(msg, args...)
}

func (a SlogAdapter) Warn(msg string, args ...any) {
	a.logger.Warn(msg, args...)
}

func NewSlogAdapter(logger *slog.Logger) Logger {
	return SlogAdapter{logger: logger}
}
