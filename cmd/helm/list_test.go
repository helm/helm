package main

import (
	"bytes"
	"testing"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestListRun(t *testing.T) {
	tests := []struct {
		name     string
		listCmd  *listCmd
		expected string
		err      bool
	}{
		{
			name: "with a release",
			listCmd: &listCmd{
				client: &fakeReleaseClient{
					rels: []*release.Release{
						releaseMock("thomas-guide"),
					},
				},
			},
			expected: "thomas-guide",
		},
		{
			name: "list --long",
			listCmd: &listCmd{
				client: &fakeReleaseClient{
					rels: []*release.Release{
						releaseMock("atlas"),
					},
				},
				long: true,
			},
			expected: "NAME \tUPDATED                 \tSTATUS  \tCHART           \natlas\tFri Sep  2 15:04:05 1977\tDEPLOYED\tfoo-0.1.0-beta.1",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		tt.listCmd.out = &buf
		err := tt.listCmd.run()
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

func TestListCmd(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flags    map[string]string
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
			expected: "thomas-guide",
		},
		{
			name:  "list --long",
			flags: map[string]string{"long": "1"},
			client: &fakeReleaseClient{
				rels: []*release.Release{
					releaseMock("atlas"),
				},
			},
			expected: "NAME \tUPDATED                 \tSTATUS  \tCHART           \natlas\tFri Sep  2 15:04:05 1977\tDEPLOYED\tfoo-0.1.0-beta.1",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		cmd := newListCmd(tt.client, &buf)
		for flag, value := range tt.flags {
			cmd.Flags().Set(flag, value)
		}
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
