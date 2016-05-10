package kube

import (
	"testing"
)

func TestAvailablePort(t *testing.T) {
	port, err := getAvailablePort()
	if err != nil {
		t.Fatal(err)
	}
	if port < 1 {
		t.Fatalf("generated port should be > 1, got %d", port)
	}
}
