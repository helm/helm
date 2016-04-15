package kube

import (
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/kubectl/resource"
)

func TestPerform(t *testing.T) {
	f, err := os.Open("./testdata/guestbook-all-in-one.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	results := []*resource.Info{}

	fn := func(info *resource.Info) error {
		results = append(results, info)

		if info.Namespace != "test" {
			t.Errorf("expected namespace to be 'test', got %s", info.Namespace)
		}

		return nil
	}

	if err := perform("test", f, fn, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if len(results) != 6 {
		t.Errorf("expected 6 result objects, got %d", len(results))
	}
}
