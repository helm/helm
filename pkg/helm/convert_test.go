package helm

import (
	"testing"

	"gopkg.in/yaml.v2"

	chartutil "k8s.io/helm/pkg/chart"
)

func TestInstallReleaseOverrides(t *testing.T) {
	// FIXME: This can't currently run unless a Tiller server is running, simply
	// because --dry-run still uses the server. There's already a WIP for a
	// testing harness, so this can be ported when that is done.
	t.Skip()

	vals := `name = "mariner"`
	ch := "./testdata/albatross"
	ir, err := InstallRelease([]byte(vals), "foo", ch, true)
	if err != nil {
		t.Fatalf("Failed to release: %s", err)
	}

	if len(ir.Release.Manifest) == 0 {
		t.Fatalf("Expected a manifest.")
	}

	// Parse the result and see if the override worked
	d := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(ir.Release.Manifest), d); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %s", err)
	}

	if d["name"] != "mariner" {
		t.Errorf("Unexpected name %q", d["name"])
	}

	if d["home"] != "nest" {
		t.Errorf("Unexpected home %q", d["home"])
	}
}

func TestOverridesToProto(t *testing.T) {
	override := []byte(`test = "foo"`)
	c := OverridesToProto(override)
	if c.Raw != string(override) {
		t.Errorf("Expected %q to match %q", c.Raw, override)
	}
}

func TestChartToProto(t *testing.T) {
	c, err := chartutil.LoadDir("./testdata/albatross")
	if err != nil {
		t.Fatalf("failed to load testdata chart: %s", err)
	}

	p, err := ChartToProto(c)
	if err != nil {
		t.Fatalf("failed to conver chart to proto: %s", err)
	}

	if p.Metadata.Name != c.Chartfile().Name {
		t.Errorf("Expected names to match.")
	}
}
