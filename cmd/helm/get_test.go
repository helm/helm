package main

import (
	"bytes"
	"testing"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestGetCmd(t *testing.T) {
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
			expected: "CHART: foo-0.1.0-beta.1\nRELEASED: Fri Sep  2 15:04:05 1977\nUSER-SUPPLIED VALUES:\nname: \"value\"\nCOMPUTED VALUES:\nname: value\n\nMANIFEST:",
		},
		{
			name:   "requires release name arg",
			client: &fakeReleaseClient{},
			err:    true,
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		cmd := newGetCmd(tt.client, &buf)
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
