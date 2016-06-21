package support

import (
	"fmt"
	"testing"
)

var _ fmt.Stringer = Message{}

func TestMessage(t *testing.T) {
	m := Message{ErrorSev, "Foo"}
	if m.String() != "[ERROR] Foo" {
		t.Errorf("Unexpected output: %s", m.String())
	}

	m = Message{WarningSev, "Bar"}
	if m.String() != "[WARNING] Bar" {
		t.Errorf("Unexpected output: %s", m.String())
	}

	m = Message{InfoSev, "FooBar"}
	if m.String() != "[INFO] FooBar" {
		t.Errorf("Unexpected output: %s", m.String())
	}
}
