package environment

import (
	"testing"

	"github.com/deis/tiller/pkg/hapi"
)

type mockEngine struct {
	out []byte
}

func (e *mockEngine) Render(chrt *hapi.Chart, v *hapi.Values) ([]byte, error) {
	return e.out, nil
}

var _ Engine = &mockEngine{}

func TestEngine(t *testing.T) {
	eng := &mockEngine{out: []byte("test")}

	env := New()
	env.EngineYard = EngineYard(map[string]Engine{"test": eng})

	if engine, ok := env.EngineYard.Get("test"); !ok {
		t.Errorf("failed to get engine from EngineYard")
	} else if out, err := engine.Render(&hapi.Chart{}, &hapi.Values{}); err != nil {
		t.Errorf("unexpected template error: %s", err)
	} else if string(out) != "test" {
		t.Errorf("expected 'test', got %q", string(out))
	}
}
