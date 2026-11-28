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

package output

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/gosuri/uitable"
)

func TestFormats(t *testing.T) {
	got := Formats()
	want := []string{"table", "json", "yaml"}
	if len(got) != len(want) {
		t.Fatalf("Formats() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Formats()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFormatsWithDesc(t *testing.T) {
	descs := FormatsWithDesc()
	for _, f := range Formats() {
		d, ok := descs[f]
		if !ok {
			t.Errorf("FormatsWithDesc() missing description for %q", f)
		}
		if d == "" {
			t.Errorf("FormatsWithDesc() has empty description for %q", f)
		}
	}
	if len(descs) != len(Formats()) {
		t.Errorf("FormatsWithDesc() has %d entries, want %d", len(descs), len(Formats()))
	}
}

func TestFormatString(t *testing.T) {
	for f, s := range map[Format]string{Table: "table", JSON: "json", YAML: "yaml"} {
		if f.String() != s {
			t.Errorf("Format(%q).String() = %q, want %q", f, f.String(), s)
		}
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{in: "table", want: Table},
		{in: "json", want: JSON},
		{in: "yaml", want: YAML},
		{in: "", want: "", wantErr: true},
		{in: "text", want: "", wantErr: true},
		{in: "Table", want: "", wantErr: true},
		{in: "YAML", want: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseFormat(tt.in)
		if tt.wantErr {
			if !errors.Is(err, ErrInvalidFormatType) {
				t.Errorf("ParseFormat(%q) error = %v, want ErrInvalidFormatType", tt.in, err)
			}
		} else if err != nil {
			t.Errorf("ParseFormat(%q) unexpected error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

type recordingWriter struct{ called string }

func (r *recordingWriter) WriteTable(_ io.Writer) error { r.called = "table"; return nil }
func (r *recordingWriter) WriteJSON(_ io.Writer) error  { r.called = "json"; return nil }
func (r *recordingWriter) WriteYAML(_ io.Writer) error  { r.called = "yaml"; return nil }

func TestFormatWrite(t *testing.T) {
	tests := []struct {
		format Format
		want   string
	}{
		{format: Table, want: "table"},
		{format: JSON, want: "json"},
		{format: YAML, want: "yaml"},
	}
	for _, tt := range tests {
		w := &recordingWriter{}
		if err := tt.format.Write(io.Discard, w); err != nil {
			t.Errorf("%q.Write() error: %v", tt.format, err)
		}
		if w.called != tt.want {
			t.Errorf("%q.Write() dispatched to %q, want %q", tt.format, w.called, tt.want)
		}
	}

	w := &recordingWriter{}
	err := Format("bogus").Write(io.Discard, w)
	if !errors.Is(err, ErrInvalidFormatType) {
		t.Errorf("bogus format error = %v, want ErrInvalidFormatType", err)
	}
	if w.called != "" {
		t.Errorf("bogus format should not dispatch, called %q", w.called)
	}
}

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) { return 0, errors.New("write failed") }

func TestEncodeJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeJSON(&buf, map[string]string{"a": "b"}); err != nil {
		t.Fatalf("EncodeJSON() error: %v", err)
	}
	if got := buf.String(); got != "{\"a\":\"b\"}\n" {
		t.Errorf("EncodeJSON() output = %q, want %q", got, "{\"a\":\"b\"}\n")
	}

	if err := EncodeJSON(failingWriter{}, map[string]string{"a": "b"}); err == nil {
		t.Error("EncodeJSON() to failing writer: expected error")
	}

	if err := EncodeJSON(io.Discard, make(chan int)); err == nil {
		t.Error("EncodeJSON() of unencodable value: expected error")
	}
}

func TestEncodeYAML(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeYAML(&buf, map[string]string{"a": "b"}); err != nil {
		t.Fatalf("EncodeYAML() error: %v", err)
	}
	if got := buf.String(); got != "a: b\n" {
		t.Errorf("EncodeYAML() output = %q, want %q", got, "a: b\n")
	}

	if err := EncodeYAML(failingWriter{}, map[string]string{"a": "b"}); err == nil {
		t.Error("EncodeYAML() to failing writer: expected error")
	}

	if err := EncodeYAML(io.Discard, make(chan int)); err == nil {
		t.Error("EncodeYAML() of unencodable value: expected error")
	}
}

func TestEncodeTable(t *testing.T) {
	table := uitable.New()
	table.AddRow("NAME", "VERSION")
	table.AddRow("nginx", "1.2.3")

	var buf bytes.Buffer
	if err := EncodeTable(&buf, table); err != nil {
		t.Fatalf("EncodeTable() error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"NAME", "VERSION", "nginx", "1.2.3"} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("EncodeTable() output missing %q: %q", want, out)
		}
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		t.Errorf("EncodeTable() output should end with a newline, got %q", out)
	}

	if err := EncodeTable(failingWriter{}, table); err == nil {
		t.Error("EncodeTable() to failing writer: expected error")
	}
}
