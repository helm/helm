package registry

import (
	"testing"
)

func TestParseType(t *testing.T) {
	// TODO: Are there some real-world examples we want to valide here?
	tests := map[string]*Type{
		"foo":                   &Type{Name: "foo"},
		"foo:v1":                &Type{Name: "foo", Version: "v1"},
		"github.com/foo":        &Type{Name: "foo", Collection: "github.com"},
		"github.com/foo:v1.2.3": &Type{Name: "foo", Collection: "github.com", Version: "v1.2.3"},
	}

	for in, expect := range tests {
		out := ParseType(in)
		if out.Name != expect.Name {
			t.Errorf("Expected name to be %q, got %q", expect.Name, out.Name)
		}
		if out.Version != expect.Version {
			t.Errorf("Expected version to be %q, got %q", expect.Version, out.Version)
		}
		if out.Collection != expect.Collection {
			t.Errorf("Expected collection to be %q, got %q", expect.Collection, out.Collection)
		}
	}
}
