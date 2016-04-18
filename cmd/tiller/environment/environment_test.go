package environment

import (
	"testing"

	"github.com/deis/tiller/pkg/hapi"
	"github.com/deis/tiller/pkg/proto/hapi/chart"
)

type mockEngine struct {
	out map[string]string
}

func (e *mockEngine) Render(chrt *chart.Chart, v *chart.Config) (map[string]string, error) {
	return e.out, nil
}

type mockReleaseStorage struct {
	rel *hapi.Release
}

func (r *mockReleaseStorage) Create(v *hapi.Release) error {
	r.rel = v
	return nil
}

func (r *mockReleaseStorage) Read(k string) (*hapi.Release, error) {
	return r.rel, nil
}

func (r *mockReleaseStorage) Update(v *hapi.Release) error {
	r.rel = v
	return nil
}

func (r *mockReleaseStorage) Delete(k string) (*hapi.Release, error) {
	return r.rel, nil
}

func (r *mockReleaseStorage) List() ([]*hapi.Release, error) {
	return []*hapi.Release{}, nil
}

func (r *mockReleaseStorage) Query(labels map[string]string) ([]*hapi.Release, error) {
	return []*hapi.Release{}, nil
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
	} else if out, err := engine.Render(&chart.Chart{}, &chart.Config{}); err != nil {
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

	if err := env.Releases.Create(release); err != nil {
		t.Fatalf("failed to store release: %s", err)
	}

	if err := env.Releases.Update(release); err != nil {
		t.Fatalf("failed to update release: %s", err)
	}

	if v, err := env.Releases.Read("albatross"); err != nil {
		t.Errorf("Error fetching release: %s", err)
	} else if v.Name != "mariner" {
		t.Errorf("Expected mariner, got %q", v.Name)
	}

	if _, err := env.Releases.Delete("albatross"); err != nil {
		t.Fatalf("failed to delete release: %s", err)
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
