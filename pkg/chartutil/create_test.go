package chartutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/helm/pkg/proto/hapi/chart"
)

func TestCreate(t *testing.T) {
	tdir, err := ioutil.TempDir("", "helm-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)

	cf := &chart.Metadata{Name: "foo"}

	c, err := Create(cf, tdir)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tdir, "foo")

	mychart, err := LoadDir(c)
	if err != nil {
		t.Fatalf("Failed to load newly created chart %q: %s", c, err)
	}

	if mychart.Metadata.Name != "foo" {
		t.Errorf("Expected name to be 'foo', got %q", mychart.Metadata.Name)
	}

	for _, d := range []string{TemplatesDir, ChartsDir} {
		if fi, err := os.Stat(filepath.Join(dir, d)); err != nil {
			t.Errorf("Expected %s dir: %s", d, err)
		} else if !fi.IsDir() {
			t.Errorf("Expected %s to be a directory.", d)
		}
	}

	for _, f := range []string{ChartfileName, ValuesfileName} {
		if fi, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("Expected %s file: %s", f, err)
		} else if fi.IsDir() {
			t.Errorf("Expected %s to be a fle.", f)
		}
	}

}
