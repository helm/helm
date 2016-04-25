package main

import (
	"bytes"
	"errors"
	"log"
	"time"

	"github.com/deis/tiller/cmd/tiller/environment"
	"github.com/deis/tiller/pkg/proto/hapi/release"
	"github.com/deis/tiller/pkg/proto/hapi/services"
	"github.com/golang/protobuf/ptypes/timestamp"
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
	// errMissingChart indicates that a chart was not provided.
	errMissingChart = errors.New("no chart provided")
	// errMissingRelease indicates that a release (name) was not provided.
	errMissingRelease = errors.New("no release provided")
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
	name := namer.NameSep("-")
	ts := now()

	// Render the templates
	files, err := s.env.EngineYard.Default().Render(req.Chart, req.Values)
	if err != nil {
		return nil, err
	}

	b := bytes.NewBuffer(nil)
	for name, file := range files {
		// Ignore empty documents because the Kubernetes library can't handle
		// them.
		if len(file) > 0 {
			b.WriteString("\n---\n# Source: " + name + "\n")
			b.WriteString(file)
		}
	}

	// Store a release.
	r := &release.Release{
		Name:   name,
		Chart:  req.Chart,
		Config: req.Values,
		Info: &release.Info{
			FirstDeployed: ts,
			LastDeployed:  ts,
			Status:        &release.Status{Code: release.Status_UNKNOWN},
		},
		Manifest: b.String(),
	}

	if req.DryRun {
		log.Printf("Dry run for %s", name)
		return &services.InstallReleaseResponse{Release: r}, nil
	}

	if err := s.env.Releases.Create(r); err != nil {
		return nil, err
	}

	return &services.InstallReleaseResponse{Release: r}, nil
}

func now() *timestamp.Timestamp {
	t := time.Now()
	ts := &timestamp.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
	return ts
}

func (s *releaseServer) UninstallRelease(c ctx.Context, req *services.UninstallReleaseRequest) (*services.UninstallReleaseResponse, error) {
	if req.Name == "" {
		log.Printf("uninstall: Release not found: %s", req.Name)
		return nil, errMissingRelease
	}

	rel, err := s.env.Releases.Read(req.Name)
	if err != nil {
		log.Printf("uninstall: Release not loaded: %s", req.Name)
		return nil, err
	}

	log.Printf("uninstall: Deleting %s", req.Name)
	rel.Info.Status.Code = release.Status_DELETED
	rel.Info.Deleted = now()

	// TODO: Once KubeClient is ready, delete the resources.
	log.Println("WARNING: Currently not deleting resources from k8s")

	if err := s.env.Releases.Update(rel); err != nil {
		log.Printf("uninstall: Failed to store updated release: %s", err)
	}

	res := services.UninstallReleaseResponse{Release: rel}
	return &res, nil
}
