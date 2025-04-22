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
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A simple handler to record the log messages emitted by slog.
// Intended to be used only by tests
type LogCaptureHandler struct {
	mu           *sync.Mutex
	records      *[]slog.Record
	opts         slog.HandlerOptions
	handlerAttrs []slog.Attr
}

type LogCaptureRecord struct {
	record slog.Record
	attrs  []slog.Attr
}

// Clones the record and associated attributes
func (l *LogCaptureRecord) Clone() LogCaptureRecord {
	copyAttrs := make([]slog.Attr, len(l.attrs))
	copy(copyAttrs, l.attrs)
	return LogCaptureRecord{
		record: l.record.Clone(),
		attrs:  copyAttrs,
	}
}

// A point in time capture of the logs emitted by slog
type LogCaptureSlice struct {
	records []LogCaptureRecord
}

// Build assertions based off the captured/filtered slice of records
type LogCaptureSliceAssertionBuilder struct {
	t *testing.T
	l *LogCaptureSlice
}

func NewLogCaptureHandler(opts *slog.HandlerOptions) *LogCaptureHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	records := make([]slog.Record, 0)
	return &LogCaptureHandler{
		mu:           &sync.Mutex{},
		records:      &records,
		opts:         *opts,
		handlerAttrs: make([]slog.Attr, 0),
	}
}

func (h *LogCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool {
	// Handle all levels
	return true
}

// Handle logs emitted by slog and capture
func (h *LogCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	r.AddAttrs(h.handlerAttrs...)
	*h.records = append(*h.records, r)

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
		records:      h.records,
		handlerAttrs: append(h.handlerAttrs, attrs...),
	}
}

// See comment on WithAttrs regarding reusing the mutex + records slice
// WithGroup is fairly unsupported in this right now as its not used in the codebase
func (h *LogCaptureHandler) WithGroup(_ string) slog.Handler {
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

// Capture returns a point in time copy of the captured log records
func (h *LogCaptureHandler) Capture() *LogCaptureSlice {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := LogCaptureSlice{
		records: make([]LogCaptureRecord, 0),
	}
	for _, record := range *h.records {
		l := LogCaptureRecord{}
		l.record = record.Clone()
		l.record.Attrs(func(a slog.Attr) bool {
			if a.Key != "" {
				l.attrs = append(l.attrs, a)
			}
			return true
		})

		result.records = append(result.records, l)
	}
	return &result
}

func (c *LogCaptureSlice) String() string {
	result := ""
	for i, record := range c.records {
		if i != 0 {
			result += "\n"
		}
		result += record.record.Message
	}

	return result
}

func (c *LogCaptureSlice) Filter(filterFn func(r *LogCaptureRecord) bool) *LogCaptureSlice {
	result := LogCaptureSlice{
		records: make([]LogCaptureRecord, 0),
	}

	for _, record := range c.records {
		if filterFn(&record) {
			result.records = append(result.records, record.Clone())
		}
	}

	return &result
}

func RecordMessageMatches(message string) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		return r.record.Message == message
	}
}

func RecordMessageContains(substring string) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		return strings.Contains(r.record.Message, substring)
	}
}

func RecordLevelMatches(level slog.Level) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		return r.record.Level == level
	}
}

func RecordLevelAboveEqual(level slog.Level) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		return r.record.Level >= level
	}
}

func RecordLevelAbove(level slog.Level) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		return r.record.Level > level
	}
}

func RecordLevelBelowEqual(level slog.Level) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		return r.record.Level <= level
	}
}

func RecordLevelBelow(level slog.Level) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		return r.record.Level < level
	}
}

func RecordHasAttr(key string) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		for _, attr := range r.attrs {
			if attr.Key == key {
				return true
			}
		}

		return false
	}
}

func RecordHasAttrValue(key string, value string) func(r *LogCaptureRecord) bool {
	return func(r *LogCaptureRecord) bool {
		for _, attr := range r.attrs {
			if attr.Key == key && attr.Value.String() == value {
				return true
			}
		}

		return false
	}
}

// Start an assertion builder for test validation
func (c *LogCaptureSlice) AssertThat(t *testing.T) *LogCaptureSliceAssertionBuilder {
	return &LogCaptureSliceAssertionBuilder{
		t: t,
		l: c,
	}
}

// Reset clears the captured log messages.
func (h *LogCaptureHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	records := make([]slog.Record, 0)

	h.records = &records
}

// Asserts that the log capture contains exactly the number of records specified
func (b *LogCaptureSliceAssertionBuilder) MatchesExactly(count int) *LogCaptureSliceAssertionBuilder {
	assert.Len(b.t, b.l.records, count, "number of records must match exactly %d record, but is %d", count, len(b.l.records))
	return b
}

// Asserts that all log records at at a specific level
func (b *LogCaptureSliceAssertionBuilder) AtLevel(level slog.Level) *LogCaptureSliceAssertionBuilder {
	if len(b.l.records) == 0 {
		assert.Fail(b.t, "expecting records to match level but there were no records to match")
	}

	for _, record := range b.l.records {
		assert.True(b.t, record.record.Level == level, "record '%s' does not match the expected level. Was %d expected %d", record.record.Message, record.record.Level, level)
	}

	return b
}

func (b *LogCaptureSliceAssertionBuilder) HasAttr(key string) *LogCaptureSliceAssertionBuilder {
	if len(b.l.records) == 0 {
		assert.Fail(b.t, "expecting records to test for attributes but there were no records to match")
	}

	for _, record := range b.l.records {

		hasAttr := false
		for _, attr := range record.attrs {
			if attr.Key == key {
				hasAttr = true
			}
		}

		assert.True(b.t, hasAttr, "record '%s' does not contain the attribute key '%s'", record.record.Message, key)
	}

	return b
}

func (b *LogCaptureSliceAssertionBuilder) HasAttrValueString(key string, value string) *LogCaptureSliceAssertionBuilder {
	if len(b.l.records) == 0 {
		assert.Fail(b.t, "expecting records to test for attributes but there were no records to match")
	}

	for _, record := range b.l.records {
		hasAttr := false
		for _, attr := range record.attrs {
			if attr.Key == key {
				hasAttr = true
				assert.True(b.t, attr.Value.String() == value, "record '%s', attribute '%s' expected value '%s' but got '%s'", record.record.Message, key, value, attr.Value.String())
			}
		}

		assert.True(b.t, hasAttr, "record '%s' does not contain the attribute key '%s'", record.record.Message, key)
	}

	return b
}
