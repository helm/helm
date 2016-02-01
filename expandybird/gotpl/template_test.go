package action

import (
	"bytes"
	"testing"
)

func TestRender(t *testing.T) {
	var b bytes.Buffer

	tpl := `{{.Hello | upper}}`
	vals := map[string]string{"Hello": "hello"}

	if err := Render(&b, tpl, vals); err != nil {
		t.Errorf("Failed to compile/render template: %s", err)
	}

	if b.String() != "HELLO" {
		t.Errorf("Expected HELLO. Got %q", b.String())
	}
}
