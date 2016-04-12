package main

import (
	"net"

	"github.com/deis/tiller/cmd/tiller/environment"
	"github.com/deis/tiller/pkg/hapi"
	ctx "golang.org/x/net/context"
	"google.golang.org/grpc"
)

type server struct {
	Environment *environment.Environment
}

// newServer creates a new server with the default environment.
//
// TODO: This can take a configuration object of some sort so that we can
// initialize the environment with the correct stuff.
func newServer() *server {
	return &server{
		Environment: environment.New(),
	}
}

func (s *server) Ready(c ctx.Context, req *hapi.PingRequest) (*hapi.PingResponse, error) {
	return &hapi.PingResponse{Status: "OK"}, nil
}

// startServer starts a new gRPC server listening on the given address.
//
// addr must conform to the requirements of "net.Listen".
func startServer(addr string) error {
	lstn, err := net.Listen("tcp", addr)
	if err != nil {
		return nil
	}

	hserver := newServer()

	srv := grpc.NewServer()
	hapi.RegisterProbeServer(srv, hserver)
	srv.Serve(lstn)

	return nil
}
