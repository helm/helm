package main

import (
	"bytes"
	"testing"

	"k8s.io/helm/pkg/helm"
)

// Stubbed out tests at two diffent layers
// TestList() is testing the command action
// TestListCmd() is testing command line interface

// TODO mock tiller responses

func TestList(t *testing.T) {
	helm.Config.ServAddr = ":44134"

	tests := []struct {
		name     string
		lister   *lister
		expected string
		err      bool
	}{
		{
			name:     "with a release",
			lister:   &lister{},
			expected: "understood-coral",
		},
		{
			name:     "list --long",
			lister:   &lister{long: true},
			expected: "NAME            \tUPDATED                 \tSTATUS  \tCHART      \nunderstood-coral\tTue Jun 28 12:29:54 2016\tDEPLOYED\tnginx-0.1.0",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		tt.lister.out = &buf
		err := tt.lister.run([]string{})
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
	helm.Config.ServAddr = ":44134"

	tests := []struct {
		name     string
		args     []string
		flags    map[string]string
		expected string
		err      bool
	}{
		{
			name:     "with a release",
			expected: "understood-coral",
		},
		{
			name:     "list --long",
			flags:    map[string]string{"long": "1"},
			expected: "NAME            \tUPDATED                 \tSTATUS  \tCHART      \nunderstood-coral\tTue Jun 28 12:29:54 2016\tDEPLOYED\tnginx-0.1.0",
		},
	}

	var buf bytes.Buffer
	for _, tt := range tests {
		cmd := newListCmd(&buf)
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
