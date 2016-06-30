package main

import (
	"bytes"
	"testing"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestGetValuesCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		client   helm.Interface
		expected string
		err      bool
	}{
		{
			name: "with a release",
			client: &fakeReleaseClient{
				rels: []*release.Release{
					releaseMock("thomas-guide"),
				},
			},
			args:     []string{"thomas-guide"},
			expected: "name: \"value\"",
		},
		{
			name:   "requires release name arg",
			client: &fakeReleaseClient{},
			err:    true,
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		cmd := newGetValuesCmd(tt.client, &buf)
		err := cmd.RunE(cmd, tt.args)
		if (err != nil) != tt.err {
			t.Errorf("%q. expected error: %v, got %v", tt.name, tt.err, err)
		}
		actual := string(bytes.TrimSpace(buf.Bytes()))
		if actual != tt.expected {
			t.Errorf("%q. expected %q, got %q", tt.name, tt.expected, actual)
		}
		buf.Reset()
	}
}
