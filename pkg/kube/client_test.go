package kube

import (
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/client/unversioned/fake"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

func TestPerform(t *testing.T) {
	input, err := os.Open("./testdata/guestbook-all-in-one.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()

	results := []*resource.Info{}

	fn := func(info *resource.Info) error {
		results = append(results, info)

		if info.Namespace != "test" {
			t.Errorf("expected namespace to be 'test', got %s", info.Namespace)
		}

		return nil
	}

	f := cmdutil.NewFactory(nil)
	f.ClientForMapping = func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
		return &fake.RESTClient{}, nil
	}

	if err := perform(f, "test", input, fn); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if len(results) != 6 {
		t.Errorf("expected 6 result objects, got %d", len(results))
	}
}
