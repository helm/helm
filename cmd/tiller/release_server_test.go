package main

import (
	"github.com/deis/tiller/cmd/tiller/environment"
	"github.com/deis/tiller/pkg/proto/hapi/chart"
	"github.com/deis/tiller/pkg/proto/hapi/services"
	"github.com/deis/tiller/pkg/storage"
	"golang.org/x/net/context"
	"testing"
)

func rsFixture() *releaseServer {
	return &releaseServer{
		env: mockEnvironment(),
	}
}

func TestInstallRelease(t *testing.T) {
	c := context.Background()
	rs := rsFixture()

	req := &services.InstallReleaseRequest{
		Chart: &chart.Chart{},
	}
	res, err := rs.InstallRelease(c, req)
	if err != nil {
		t.Errorf("Failed install: %s", err)
	}
	if res.Release.Name == "" {
		t.Errorf("Expected release name.")
	}
}

func mockEnvironment() *environment.Environment {
	e := environment.New()
	e.Releases = storage.NewMemory()
	return e
}
