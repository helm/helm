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
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogHolder_Logger(t *testing.T) {
	t.Run("should return new logger with a then set handler", func(t *testing.T) {
		holder := &LogHolder{}
		buf := &bytes.Buffer{}
		handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})

		holder.SetLogger(handler)
		logger := holder.Logger()

		assert.NotNil(t, logger)

		// Test that the logger works
		logger.Info("test message")
		assert.Contains(t, buf.String(), "test message")
	})

	t.Run("should return discard - defaultlogger when no handler is set", func(t *testing.T) {
		holder := &LogHolder{}
		logger := holder.Logger()

		assert.Equal(t, slog.Handler(slog.DiscardHandler), logger.Handler())
	})
}

func TestLogHolder_SetLogger(t *testing.T) {
	t.Run("sets logger with valid handler", func(t *testing.T) {
		holder := &LogHolder{}
		buf := &bytes.Buffer{}
		handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})

		holder.SetLogger(handler)
		logger := holder.Logger()

		assert.NotNil(t, logger)

		// Compare the handler directly
		assert.Equal(t, handler, logger.Handler())
	})

	t.Run("sets discard logger with nil handler", func(t *testing.T) {
		holder := &LogHolder{}

		holder.SetLogger(nil)
		logger := holder.Logger()

		assert.NotNil(t, logger)

		assert.Equal(t, slog.Handler(slog.DiscardHandler), logger.Handler())
	})

	t.Run("can replace existing logger", func(t *testing.T) {
		holder := &LogHolder{}

		// Set first logger
		buf1 := &bytes.Buffer{}
		handler1 := slog.NewTextHandler(buf1, &slog.HandlerOptions{Level: slog.LevelDebug})
		holder.SetLogger(handler1)

		logger1 := holder.Logger()
		assert.Equal(t, handler1, logger1.Handler())

		// Replace with second logger
		buf2 := &bytes.Buffer{}
		handler2 := slog.NewTextHandler(buf2, &slog.HandlerOptions{Level: slog.LevelDebug})
		holder.SetLogger(handler2)

		logger2 := holder.Logger()
		assert.Equal(t, handler2, logger2.Handler())
	})
}

func TestLogHolder_InterfaceCompliance(t *testing.T) {
	t.Run("implements LoggerSetterGetter interface", func(_ *testing.T) {
		var _ LoggerSetterGetter = &LogHolder{}
	})

	t.Run("interface methods work correctly", func(t *testing.T) {
		var holder LoggerSetterGetter = &LogHolder{}

		buf := &bytes.Buffer{}
		handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})

		holder.SetLogger(handler)
		logger := holder.Logger()

		assert.NotNil(t, logger)
		assert.Equal(t, handler, logger.Handler())
	})
}

func TestDebugCheckHandler_Enabled(t *testing.T) {
	t.Run("returns debugEnabled function result for debug level", func(t *testing.T) {
		// Test with debug enabled
		debugEnabled := func() bool { return true }
		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: debugEnabled,
		}

		assert.True(t, handler.Enabled(t.Context(), slog.LevelDebug))
	})

	t.Run("returns false for debug level when debug disabled", func(t *testing.T) {
		// Test with debug disabled
		debugEnabled := func() bool { return false }
		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: debugEnabled,
		}

		assert.False(t, handler.Enabled(t.Context(), slog.LevelDebug))
	})

	t.Run("always returns true for non-debug levels", func(t *testing.T) {
		debugEnabled := func() bool { return false } // Debug disabled
		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: debugEnabled,
		}

		// Even with debug disabled, other levels should always be enabled
		assert.True(t, handler.Enabled(t.Context(), slog.LevelInfo))
		assert.True(t, handler.Enabled(t.Context(), slog.LevelWarn))
		assert.True(t, handler.Enabled(t.Context(), slog.LevelError))
	})

	t.Run("calls debugEnabled function dynamically", func(t *testing.T) {
		callCount := 0
		debugEnabled := func() bool {
			callCount++
			return callCount%2 == 1 // Alternates between true and false
		}

		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: debugEnabled,
		}

		// First call should return true
		assert.True(t, handler.Enabled(t.Context(), slog.LevelDebug))
		assert.Equal(t, 1, callCount)

		// Second call should return false
		assert.False(t, handler.Enabled(t.Context(), slog.LevelDebug))
		assert.Equal(t, 2, callCount)

		// Third call should return true again
		assert.True(t, handler.Enabled(t.Context(), slog.LevelDebug))
		assert.Equal(t, 3, callCount)
	})
}

func TestDebugCheckHandler_Handle(t *testing.T) {
	t.Run("delegates to underlying handler", func(t *testing.T) {
		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: func() bool { return true },
		}

		record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
		err := handler.Handle(t.Context(), record)

		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "test message")
	})

	t.Run("handles context correctly", func(t *testing.T) {
		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: func() bool { return true },
		}

		type testKey string
		ctx := context.WithValue(t.Context(), testKey("test"), "value")
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "context test", 0)
		err := handler.Handle(ctx, record)

		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "context test")
	})
}

func TestDebugCheckHandler_WithAttrs(t *testing.T) {
	t.Run("returns new DebugCheckHandler with attributes", func(t *testing.T) {
		logger := NewLogger(func() bool { return true })
		handler := logger.Handler()
		newHandler := handler.WithAttrs([]slog.Attr{
			slog.String("key1", "value1"),
			slog.Int("key2", 42),
		})

		// Should return a DebugCheckHandler
		debugHandler, ok := newHandler.(*DebugCheckHandler)
		assert.True(t, ok)
		assert.NotNil(t, debugHandler)

		// Should preserve the debugEnabled function
		assert.True(t, debugHandler.Enabled(t.Context(), slog.LevelDebug))

		// Should have the attributes applied to the underlying handler
		assert.NotEqual(t, handler, debugHandler.handler)
	})

	t.Run("preserves debugEnabled function", func(t *testing.T) {
		callCount := 0
		debugEnabled := func() bool {
			callCount++
			return callCount%2 == 1
		}

		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: debugEnabled,
		}

		attrs := []slog.Attr{slog.String("test", "value")}
		newHandler := handler.WithAttrs(attrs)

		// The new handler should use the same debugEnabled function
		assert.True(t, newHandler.Enabled(t.Context(), slog.LevelDebug))
		assert.Equal(t, 1, callCount)

		assert.False(t, newHandler.Enabled(t.Context(), slog.LevelDebug))
		assert.Equal(t, 2, callCount)
	})
}

func TestDebugCheckHandler_WithGroup(t *testing.T) {
	t.Run("returns new DebugCheckHandler with group", func(t *testing.T) {
		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: func() bool { return true },
		}

		newHandler := handler.WithGroup("testgroup")

		// Should return a DebugCheckHandler
		debugHandler, ok := newHandler.(*DebugCheckHandler)
		assert.True(t, ok)
		assert.NotNil(t, debugHandler)

		// Should preserve the debugEnabled function
		assert.True(t, debugHandler.Enabled(t.Context(), slog.LevelDebug))

		// Should have the group applied to the underlying handler
		assert.NotEqual(t, handler.handler, debugHandler.handler)
	})

	t.Run("preserves debugEnabled function", func(t *testing.T) {
		callCount := 0
		debugEnabled := func() bool {
			callCount++
			return callCount%2 == 1
		}

		buf := &bytes.Buffer{}
		baseHandler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
		handler := &DebugCheckHandler{
			handler:      baseHandler,
			debugEnabled: debugEnabled,
		}

		newHandler := handler.WithGroup("testgroup")

		// The new handler should use the same debugEnabled function
		assert.True(t, newHandler.Enabled(t.Context(), slog.LevelDebug))
		assert.Equal(t, 1, callCount)

		assert.False(t, newHandler.Enabled(t.Context(), slog.LevelDebug))
		assert.Equal(t, 2, callCount)
	})
}

func TestDebugCheckHandler_Integration(t *testing.T) {
	t.Run("works with NewLogger function", func(t *testing.T) {
		debugEnabled := func() bool { return true }
		logger := NewLogger(debugEnabled)

		assert.NotNil(t, logger)

		// The logger should have a DebugCheckHandler
		handler := logger.Handler()
		debugHandler, ok := handler.(*DebugCheckHandler)
		assert.True(t, ok)

		// Should enable debug when debugEnabled returns true
		assert.True(t, debugHandler.Enabled(t.Context(), slog.LevelDebug))

		// Should enable other levels regardless
		assert.True(t, debugHandler.Enabled(t.Context(), slog.LevelInfo))
	})

	t.Run("dynamic debug checking works in practice", func(t *testing.T) {
		debugState := false
		debugEnabled := func() bool { return debugState }

		logger := NewLogger(debugEnabled)

		// Initially debug should be disabled
		assert.False(t, logger.Handler().(*DebugCheckHandler).Enabled(t.Context(), slog.LevelDebug))

		// Enable debug
		debugState = true
		assert.True(t, logger.Handler().(*DebugCheckHandler).Enabled(t.Context(), slog.LevelDebug))

		// Disable debug again
		debugState = false
		assert.False(t, logger.Handler().(*DebugCheckHandler).Enabled(t.Context(), slog.LevelDebug))
	})

	t.Run("handles nil debugEnabled function", func(t *testing.T) {
		logger := NewLogger(nil)

		assert.NotNil(t, logger)

		// The logger should have a DebugCheckHandler
		handler := logger.Handler()
		debugHandler, ok := handler.(*DebugCheckHandler)
		assert.True(t, ok)

		// When debugEnabled is nil, debug level should be disabled (default behavior)
		assert.False(t, debugHandler.Enabled(t.Context(), slog.LevelDebug))

		// Other levels should always be enabled
		assert.True(t, debugHandler.Enabled(t.Context(), slog.LevelInfo))
		assert.True(t, debugHandler.Enabled(t.Context(), slog.LevelWarn))
		assert.True(t, debugHandler.Enabled(t.Context(), slog.LevelError))
	})
}
