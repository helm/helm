package main

import (
	"errors"

	"github.com/deis/tiller/cmd/tiller/environment"
	"github.com/deis/tiller/pkg/proto/hapi/services"
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

// errNotImplemented is a temporary error for uninmplemented callbacks.
var errNotImplemented = errors.New("not implemented")

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
	return &services.InstallReleaseResponse{}, errNotImplemented
}

func (s *releaseServer) UninstallRelease(c ctx.Context, req *services.UninstallReleaseRequest) (*services.UninstallReleaseResponse, error) {
	return nil, errNotImplemented
}
