/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
