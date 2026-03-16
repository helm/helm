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

package logging

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
)

// DebugEnabledFunc is a function type that determines if debug logging is enabled
// We use a function because we want to check the setting at log time, not when the logger is created
type DebugEnabledFunc func() bool

// DebugCheckHandler checks settings.Debug at log time
type DebugCheckHandler struct {
	handler      slog.Handler
	debugEnabled DebugEnabledFunc
}

// Enabled implements slog.Handler.Enabled
func (h *DebugCheckHandler) Enabled(_ context.Context, level slog.Level) bool {
	if level == slog.LevelDebug {
		if h.debugEnabled == nil {
			return false
		}
		return h.debugEnabled()
	}
	return true // Always log other levels
}

// Handle implements slog.Handler.Handle
func (h *DebugCheckHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.WithAttrs
func (h *DebugCheckHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &DebugCheckHandler{
		handler:      h.handler.WithAttrs(attrs),
		debugEnabled: h.debugEnabled,
	}
}

// WithGroup implements slog.Handler.WithGroup
func (h *DebugCheckHandler) WithGroup(name string) slog.Handler {
	return &DebugCheckHandler{
		handler:      h.handler.WithGroup(name),
		debugEnabled: h.debugEnabled,
	}
}

// NewLogger creates a new logger with dynamic debug checking
func NewLogger(debugEnabled DebugEnabledFunc) *slog.Logger {
	// Create base handler that removes timestamps
	baseHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		// Always use LevelDebug here to allow all messages through
		// Our custom handler will do the filtering
		Level: slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Remove the time attribute
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})

	// Wrap with our dynamic debug-checking handler
	dynamicHandler := &DebugCheckHandler{
		handler:      baseHandler,
		debugEnabled: debugEnabled,
	}

	return slog.New(dynamicHandler)
}

// LoggerSetterGetter is an interface that can set and get a logger
type LoggerSetterGetter interface {
	// SetLogger sets a new slog.Handler
	SetLogger(newHandler slog.Handler)
	// Logger returns the slog.Logger created from the slog.Handler
	Logger() *slog.Logger
}

type LogHolder struct {
	// logger is an atomic.Pointer[slog.Logger] to store the slog.Logger
	// We use atomic.Pointer for thread safety
	logger atomic.Pointer[slog.Logger]
}

// Logger returns the logger for the LogHolder. If nil, returns slog.Default().
func (l *LogHolder) Logger() *slog.Logger {
	if lg := l.logger.Load(); lg != nil {
		return lg
	}
	return slog.New(slog.DiscardHandler) // Should never be reached
}

// SetLogger sets the logger for the LogHolder. If nil, sets the default logger.
func (l *LogHolder) SetLogger(newHandler slog.Handler) {
	if newHandler == nil {
		l.logger.Store(slog.New(slog.DiscardHandler)) // Assume nil as discarding logs
		return
	}
	l.logger.Store(slog.New(newHandler))
}

// Ensure LogHolder implements LoggerSetterGetter
var _ LoggerSetterGetter = &LogHolder{}
