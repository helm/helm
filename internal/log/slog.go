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

import (
	"io"
	"log/slog"
)

// SlogAdapter adapts a standard library slog.Logger to the Logger interface.
type SlogAdapter struct {
	logger *slog.Logger
}

// Debug implements Logger.Debug by forwarding to the underlying slog.Logger.
func (a SlogAdapter) Debug(msg string, args ...any) {
	a.logger.Debug(msg, args...)
}

// Warn implements Logger.Warn by forwarding to the underlying slog.Logger.
func (a SlogAdapter) Warn(msg string, args ...any) {
	a.logger.Warn(msg, args...)
}

// Error implements Logger.Error by forwarding to the underlying slog.Logger.
func (a SlogAdapter) Error(msg string, args ...any) {
	// TODO: Handle error with `slog.Any`: slog.Info("something went wrong", slog.Any("err", err))
	a.logger.Error(msg, args...)
}

// NewSlogAdapter creates a Logger that forwards log messages to a slog.Logger.
func NewSlogAdapter(logger *slog.Logger) Logger {
	if logger == nil {
		return DefaultLogger
	}
	return SlogAdapter{logger: logger}
}

// NewReadableTextLogger creates a Logger that outputs in a readable text format without timestamps
func NewReadableTextLogger(output io.Writer, debugEnabled bool) Logger {
	level := slog.LevelInfo
	if debugEnabled {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})

	return NewSlogAdapter(slog.New(handler))
}
