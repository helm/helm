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

package util

import (
	"context"
	"log/slog"
	"sync"
)

// A simple handler to record the log messages emitted by slog.
// Intended to be used only by tests
type LogCaptureHandler struct {
	mu      *sync.Mutex
	records []slog.Record
	opts    slog.HandlerOptions
}

type LogCaptureSlice []slog.Record

func NewLogCaptureHandler(opts *slog.HandlerOptions) *LogCaptureHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &LogCaptureHandler{
		mu:   &sync.Mutex{},
		opts: *opts,
	}
}

func (h *LogCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	// Handle all levels
	return true
}

func (h *LogCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

// This is intentionally not following "standard" usage of WithAttrs and WithGroup with regard
// to handlers since we want to be able to capture all messages emitted from slog sources rather
// than each sub-logger generating its own slice of records
func (h *LogCaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogCaptureHandler{
		// Copy the mutex as we could be writing to the same records from multiple threads
		mu: h.mu,
		opts: slog.HandlerOptions{
			Level:       h.opts.Level,
			AddSource:   h.opts.AddSource,
			ReplaceAttr: h.opts.ReplaceAttr,
		},

		// Copy the same records as we want to be able to collate _all_ the log messages
		// emitted by slog sources
		records: h.records,
	}
}

// See comment on WithAttrs regarding reusing the mutex + records slice
func (h *LogCaptureHandler) WithGroup(name string) slog.Handler {
	return &LogCaptureHandler{
		mu: h.mu,
		opts: slog.HandlerOptions{
			Level:       h.opts.Level,
			AddSource:   h.opts.AddSource,
			ReplaceAttr: h.opts.ReplaceAttr,
		},

		// Copy the same records as we want to be able to collate _all_ the log messages
		// emitted by slog sources
		records: h.records,
	}
}

// Records returns a copy of the captured log records
func (h *LogCaptureHandler) Records() *LogCaptureSlice {
	h.mu.Lock()
	defer h.mu.Unlock()

	records := make([]slog.Record, len(h.records))
	copy(records, h.records)

	result := LogCaptureSlice(records)
	return &result
}

// Converts the captured log records into a map of log message and log level
func (l *LogCaptureSlice) AsMessageLevelMap() map[string]slog.Level {

	result := make(map[string]slog.Level)
	for _, record := range *l {
		result[record.Message] = record.Level
	}

	return result
}

// Converts the captured log records into a slice of log messages
func (l *LogCaptureSlice) AsMessageSlice() []string {

	result := make([]string, 0)
	for _, record := range *l {
		result = append(result, record.Message)
	}

	return result
}

// Reset clears the captured log messages.
func (h *LogCaptureHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = make([]slog.Record, 0)
}
