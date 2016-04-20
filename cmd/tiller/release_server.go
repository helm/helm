package main

import (
	"errors"

	"github.com/deis/tiller/cmd/tiller/environment"
	"github.com/deis/tiller/pkg/proto/hapi/release"
	"github.com/deis/tiller/pkg/proto/hapi/services"
	"github.com/technosophos/moniker"
	ctx "golang.org/x/net/context"
)

func init() {
	srv := &releaseServer{
		env: env,
	}
	services.RegisterReleaseServiceServer(rootServer, srv)
}

type releaseServer struct {
	env *environment.Environment
}

var (
	// errNotImplemented is a temporary error for uninmplemented callbacks.
	errNotImplemented = errors.New("not implemented")
	errMissingChart   = errors.New("no chart provided")
)

func (s *releaseServer) ListReleases(req *services.ListReleasesRequest, stream services.ReleaseService_ListReleasesServer) error {
	return errNotImplemented
}

func (s *releaseServer) GetReleaseStatus(c ctx.Context, req *services.GetReleaseStatusRequest) (*services.GetReleaseStatusResponse, error) {
	return nil, errNotImplemented
}

func (s *releaseServer) GetReleaseContent(c ctx.Context, req *services.GetReleaseContentRequest) (*services.GetReleaseContentResponse, error) {
	return nil, errNotImplemented
}

func (s *releaseServer) UpdateRelease(c ctx.Context, req *services.UpdateReleaseRequest) (*services.UpdateReleaseResponse, error) {
	return nil, errNotImplemented
}

func (s *releaseServer) InstallRelease(c ctx.Context, req *services.InstallReleaseRequest) (*services.InstallReleaseResponse, error) {
	if req.Chart == nil {
		return nil, errMissingChart
	}

	// We should probably make a name generator part of the Environment.
	namer := moniker.New()
	// TODO: Make sure this is unique.
	name := namer.Name()

	// Render the templates
	_, err := s.env.EngineYard.Default().Render(req.Chart, req.Values)
	if err != nil {
		return nil, err
	}

	// Store a release.
	r := &release.Release{
		Name:   name,
		Chart:  req.Chart,
		Config: req.Values,
		Info: &release.Info{
			Status: &release.Status{Code: release.Status_UNKNOWN},
		},
	}

	if err := s.env.Releases.Create(r); err != nil {
		return nil, err
	}

	return &services.InstallReleaseResponse{Release: r}, nil
}

func (s *releaseServer) UninstallRelease(c ctx.Context, req *services.UninstallReleaseRequest) (*services.UninstallReleaseResponse, error) {
	return nil, errNotImplemented
}
