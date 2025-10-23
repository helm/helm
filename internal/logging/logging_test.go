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
	"log/slog"
	"testing"

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
