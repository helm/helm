package main

import (
	"net"

	"github.com/deis/tiller/pkg/hapi"
	ctx "golang.org/x/net/context"
	"google.golang.org/grpc"
)

type server struct{}

func (s *server) Ready(c ctx.Context, req *hapi.PingRequest) (*hapi.PingResponse, error) {
	return &hapi.PingResponse{Status: "OK"}, nil
}

func startServer(addr string) error {
	lstn, err := net.Listen("tcp", addr)
	if err != nil {
		return nil
	}

	hserver := &server{}

	srv := grpc.NewServer()
	hapi.RegisterProbeServer(srv, hserver)
	srv.Serve(lstn)

	return nil
}
