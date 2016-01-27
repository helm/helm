package log

import (
	"bytes"
	"fmt"
	"testing"
)

type LoggerMock struct {
	b bytes.Buffer
}

func (l *LoggerMock) Printf(m string, v ...interface{}) {
	l.b.Write([]byte(fmt.Sprintf(m, v...)))
}

func TestLogger(t *testing.T) {
	l := &LoggerMock{}
	Logger = l
	IsDebugging = true

	Err("%s%s%s", "a", "b", "c")
	expect := "[ERROR] abc\n"
	if l.b.String() != expect {
		t.Errorf("Expected %q, got %q", expect, l.b.String())
	}
	l.b.Reset()

	tests := map[string]func(string, ...interface{}){
		"[WARN] test\n":  Warn,
		"[INFO] test\n":  Info,
		"[DEBUG] test\n": Debug,
	}

	for expect, f := range tests {
		f("test")
		if l.b.String() != expect {
			t.Errorf("Expected %q, got %q", expect, l.b.String())
		}
		l.b.Reset()
	}

	IsDebugging = false
	Debug("HELLO")
	if l.b.String() != "" {
		t.Errorf("Expected debugging to disable. Got %q", l.b.String())
	}
	l.b.Reset()
}
