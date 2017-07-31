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

package tiller

import (
	"golang.org/x/net/context"
	"k8s.io/helm/pkg/proto/hapi/kube"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

// InfoServer implements the server-side gRPC endpoint for the HAPI info service.
type InfoServer struct {
	clientset internalclientset.Interface
	env       *environment.Environment
}

// GetKubeInfo returns Kubernetes server information.
func (s *InfoServer) GetKubeInfo(context.Context, *services.GetKubeInfoRequest) (*services.GetKubeInfoResponse, error) {
	v, err := s.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	i := &kube.Info{
		Version: &kube.Version{
			Major:        v.Major,
			Minor:        v.Minor,
			GitVersion:   v.GitVersion,
			GitCommit:    v.GitCommit,
			GitTreeState: v.GitTreeState,
			BuildDate:    v.BuildDate,
			GoVersion:    v.GoVersion,
			Compiler:     v.Compiler,
			Platform:     v.Platform,
		},
	}
	return &services.GetKubeInfoResponse{Info: i}, nil
}

// NewInfoServer returns a new InfoServer.
func NewInfoServer(env *environment.Environment, clientset internalclientset.Interface) *InfoServer {
	return &InfoServer{
		env:       env,
		clientset: clientset,
	}
}
