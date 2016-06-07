package environment

import (
	"bytes"
	"io"
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
)

type mockEngine struct {
	out map[string]string
}

func (e *mockEngine) Render(chrt *chart.Chart, v *chart.Config, o map[string]interface{}) (map[string]string, error) {
	return e.out, nil
}

type mockReleaseStorage struct {
	rel *release.Release
}

func (r *mockReleaseStorage) Create(v *release.Release) error {
	r.rel = v
	return nil
}

func (r *mockReleaseStorage) Read(k string) (*release.Release, error) {
	return r.rel, nil
}

func (r *mockReleaseStorage) Update(v *release.Release) error {
	r.rel = v
	return nil
}

func (r *mockReleaseStorage) Delete(k string) (*release.Release, error) {
	return r.rel, nil
}

func (r *mockReleaseStorage) List() ([]*release.Release, error) {
	return []*release.Release{}, nil
}

func (r *mockReleaseStorage) Query(labels map[string]string) ([]*release.Release, error) {
	return []*release.Release{}, nil
}

func (r *mockReleaseStorage) History(n string) ([]*release.Release, error) {
	res := []*release.Release{}
	rel, err := r.Read(n)
	if err != nil {
		return res, err
	}
	res = append(res, rel)
	return res, nil
}

type mockKubeClient struct {
}

func (k *mockKubeClient) Create(ns string, r io.Reader) error {
	return nil
}
func (k *mockKubeClient) Delete(ns string, r io.Reader) error {
	return nil
}

var _ Engine = &mockEngine{}
var _ ReleaseStorage = &mockReleaseStorage{}
var _ KubeClient = &mockKubeClient{}
var _ KubeClient = &PrintingKubeClient{}

func TestEngine(t *testing.T) {
	eng := &mockEngine{out: map[string]string{"albatross": "test"}}

	env := New()
	env.EngineYard = EngineYard(map[string]Engine{"test": eng})

	if engine, ok := env.EngineYard.Get("test"); !ok {
		t.Errorf("failed to get engine from EngineYard")
	} else if out, err := engine.Render(&chart.Chart{}, &chart.Config{}, map[string]interface{}{}); err != nil {
		t.Errorf("unexpected template error: %s", err)
	} else if out["albatross"] != "test" {
		t.Errorf("expected 'test', got %q", out["albatross"])
	}
}

func TestReleaseStorage(t *testing.T) {
	rs := &mockReleaseStorage{}
	env := New()
	env.Releases = rs

	release := &release.Release{Name: "mariner"}

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

	manifests := map[string]string{
		"foo": "name: value\n",
		"bar": "name: value\n",
	}

	b := bytes.NewBuffer(nil)
	for _, content := range manifests {
		b.WriteString("\n---\n")
		b.WriteString(content)
	}

	if err := env.KubeClient.Create("sharry-bobbins", b); err != nil {
		t.Errorf("Kubeclient failed: %s", err)
	}
}
