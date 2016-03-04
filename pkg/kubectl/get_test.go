package kubectl

import (
	"testing"
)

func TestGet(t *testing.T) {
	Client = TestRunner{
		out: []byte("running the get command"),
	}

	expects := "running the get command"
	out, _ := Client.Get([]byte{}, "")
	if string(out) != expects {
		t.Errorf("%s != %s", string(out), expects)
	}
}

func TestGetByKind(t *testing.T) {
	Client = TestRunner{
		out: []byte("running the GetByKind command"),
	}

	expects := "running the GetByKind command"
	out, _ := Client.GetByKind("pods", "", "")
	if out != expects {
		t.Errorf("%s != %s", out, expects)
	}
}
