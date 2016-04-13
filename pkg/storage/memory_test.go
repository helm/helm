package storage

import (
	"testing"

	"github.com/deis/tiller/pkg/hapi"
)

func TestSet(t *testing.T) {
	k := "test-1"
	r := &hapi.Release{Name: k}

	ms := NewMemory()
	if err := ms.Set(k, r); err != nil {
		t.Fatalf("Failed set: %s", err)
	}

	if ms.releases[k].Name != k {
		t.Errorf("Unexpected release name: %s", ms.releases[k].Name)
	}
}

func TestGet(t *testing.T) {
	k := "test-1"
	r := &hapi.Release{Name: k}

	ms := NewMemory()
	ms.Set(k, r)

	if out, err := ms.Get(k); err != nil {
		t.Errorf("Could not get %s: %s", k, err)
	} else if out.Name != k {
		t.Errorf("Expected %s, got %s", k, out.Name)
	}
}

func TestList(t *testing.T) {
	ms := NewMemory()
	rels := []string{"a", "b", "c"}

	for _, k := range rels {
		ms.Set(k, &hapi.Release{Name: k})
	}

	l, err := ms.List()
	if err != nil {
		t.Error(err)
	}

	if len(l) != 3 {
		t.Errorf("Expected 3, got %d", len(l))
	}

	for _, n := range rels {
		foundN := false
		for _, rr := range l {
			if rr.Name == n {
				foundN = true
				break
			}
		}
		if !foundN {
			t.Errorf("Did not find %s in list.", n)
		}
	}
}

func TestQuery(t *testing.T) {
	t.Skip("Not Implemented")
}
