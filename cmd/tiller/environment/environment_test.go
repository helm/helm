package environment

import (
	"testing"

	"github.com/deis/tiller/pkg/hapi"
)

type mockEngine struct {
	out map[string]string
}

func (e *mockEngine) Render(chrt *hapi.Chart, v *hapi.Values) (map[string]string, error) {
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

func (k *mockKubeClient) Install(manifests map[string]string) error {
	return nil
}

var _ Engine = &mockEngine{}
var _ ReleaseStorage = &mockReleaseStorage{}
var _ KubeClient = &mockKubeClient{}

func TestEngine(t *testing.T) {
	eng := &mockEngine{out: map[string]string{"albatross": "test"}}

	env := New()
	env.EngineYard = EngineYard(map[string]Engine{"test": eng})

	if engine, ok := env.EngineYard.Get("test"); !ok {
		t.Errorf("failed to get engine from EngineYard")
	} else if out, err := engine.Render(&hapi.Chart{}, &hapi.Values{}); err != nil {
		t.Errorf("unexpected template error: %s", err)
	} else if out["albatross"] != "test" {
		t.Errorf("expected 'test', got %q", out["albatross"])
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

	manifests := map[string]string{}

	if err := env.KubeClient.Install(manifests); err != nil {
		t.Errorf("Kubeclient failed: %s", err)
	}
}
