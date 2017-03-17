/*
Copyright 2017 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main // import "k8s.io/helm/cmd/rudder"

import (
	"bytes"
	"fmt"
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"

	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/rudder"
	"k8s.io/helm/pkg/version"
)

var kubeClient = kube.New(nil)

func main() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", rudder.GrpcPort))
	if err != nil {
		grpclog.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	release.RegisterReleaseModuleServiceServer(grpcServer, &ReleaseModuleServiceServer{})

	grpclog.Print("Server starting")
	grpcServer.Serve(lis)
	grpclog.Print("Server started")
}

// ReleaseModuleServiceServer provides implementation for release.ReleaseModuleServiceServer
type ReleaseModuleServiceServer struct{}

// Version is not yet implemented
func (r *ReleaseModuleServiceServer) Version(ctx context.Context, in *release.VersionReleaseRequest) (*release.VersionReleaseResponse, error) {
	grpclog.Print("version")
	return &release.VersionReleaseResponse{
		Name:    "helm-rudder-native",
		Version: version.Version,
	}, nil
}

// InstallRelease creates a release using kubeClient.Create
func (r *ReleaseModuleServiceServer) InstallRelease(ctx context.Context, in *release.InstallReleaseRequest) (*release.InstallReleaseResponse, error) {
	grpclog.Print("install")
	b := bytes.NewBufferString(in.Release.Manifest)
	err := kubeClient.Create(in.Release.Namespace, b, 500, false)
	if err != nil {
		grpclog.Printf("error when creating release: %s", err)
	}
	return &release.InstallReleaseResponse{}, err
}

// DeleteRelease is not implemented
func (r *ReleaseModuleServiceServer) DeleteRelease(ctx context.Context, in *release.DeleteReleaseRequest) (*release.DeleteReleaseResponse, error) {
	grpclog.Print("delete")
	return nil, nil
}

// RollbackRelease is not implemented
func (r *ReleaseModuleServiceServer) RollbackRelease(ctx context.Context, in *release.RollbackReleaseRequest) (*release.RollbackReleaseResponse, error) {
	grpclog.Print("rollback")
	c := bytes.NewBufferString(in.Current.Manifest)
	t := bytes.NewBufferString(in.Target.Manifest)
	err := kubeClient.Update(in.Target.Namespace, c, t, in.Recreate, in.Timeout, in.Wait)
	return &release.RollbackReleaseResponse{}, err
}

// UpgradeRelease upgrades manifests using kubernetes client
func (r *ReleaseModuleServiceServer) UpgradeRelease(ctx context.Context, in *release.UpgradeReleaseRequest) (*release.UpgradeReleaseResponse, error) {
	grpclog.Print("upgrade")
	c := bytes.NewBufferString(in.Current.Manifest)
	t := bytes.NewBufferString(in.Target.Manifest)
	err := kubeClient.Update(in.Target.Namespace, c, t, in.Recreate, in.Timeout, in.Wait)
	// upgrade response object should be changed to include status
	return &release.UpgradeReleaseResponse{}, err
}
