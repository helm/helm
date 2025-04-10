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

package cli

import (
	"context"
	"log/slog"
	"os"
)

// DebugCheckHandler checks settings.Debug at log time
type DebugCheckHandler struct {
	handler  slog.Handler
	settings *EnvSettings
}

// Enabled implements slog.Handler.Enabled
func (h *DebugCheckHandler) Enabled(_ context.Context, level slog.Level) bool {
	if level == slog.LevelDebug {
		return h.settings.Debug // Check settings.Debug at log time
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
		handler:  h.handler.WithAttrs(attrs),
		settings: h.settings,
	}
}

// WithGroup implements slog.Handler.WithGroup
func (h *DebugCheckHandler) WithGroup(name string) slog.Handler {
	return &DebugCheckHandler{
		handler:  h.handler.WithGroup(name),
		settings: h.settings,
	}
}

// NewLogger creates a new logger with dynamic debug checking
func NewLogger(settings *EnvSettings) *slog.Logger {
	// Create base handler that removes timestamps
	baseHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
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
		handler:  baseHandler,
		settings: settings,
	}

	return slog.New(dynamicHandler)
}
