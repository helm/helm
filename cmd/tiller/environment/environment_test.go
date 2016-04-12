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

type mockReleaseStorage struct {
	rel *hapi.Release
}

func (r *mockReleaseStorage) Get(k string) (*hapi.Release, error) {
	return r.rel, nil
}

func (r *mockReleaseStorage) Set(k string, v *hapi.Release) error {
	r.rel = v
	return nil
}

type mockKubeClient struct {
}

func (k *mockKubeClient) Install(manifest []byte) error {
	return nil
}

var _ Engine = &mockEngine{}
var _ ReleaseStorage = &mockReleaseStorage{}
var _ KubeClient = &mockKubeClient{}

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

func TestReleaseStorage(t *testing.T) {
	rs := &mockReleaseStorage{}
	env := New()
	env.Releases = rs

	release := &hapi.Release{Name: "mariner"}

	if err := env.Releases.Set("albatross", release); err != nil {
		t.Fatalf("failed to store release: %s", err)
	}

	if v, err := env.Releases.Get("albatross"); err != nil {
		t.Errorf("Error fetching release: %s", err)
	} else if v.Name != "mariner" {
		t.Errorf("Expected mariner, got %q", v.Name)
	}
}

func TestKubeClient(t *testing.T) {
	kc := &mockKubeClient{}
	env := New()
	env.KubeClient = kc

	if err := env.KubeClient.Install([]byte("apiVersion: v1\n")); err != nil {
		t.Errorf("Kubeclient failed: %s", err)
	}
}
