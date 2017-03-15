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

package rudder // import "k8s.io/helm/pkg/rudder"

import (
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"k8s.io/helm/pkg/proto/hapi/release"
)

// GrpcAddr is port number for accessing Rudder service
var GrpcAddr = 10001

// InstallRelease calls Rudder InstallRelease method which should create provided release
func InstallRelease(rel *release.InstallReleaseRequest) (*release.InstallReleaseResponse, error) {
	//TODO(mkwiek): parametrize this
	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", GrpcAddr), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	defer conn.Close()
	client := release.NewReleaseModuleServiceClient(conn)
	return client.InstallRelease(context.Background(), rel)
}

// UpgradeReleas calls Rudder UpgradeRelease method which should perform update
func UpgradeRelease(req *release.UpgradeReleaseRequest) (*release.UpgradeReleaseResponse, error) {
	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%d", GrpcAddr), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := release.NewReleaseModuleServiceClient(conn)
	return client.UpgradeRelease(context.Background(), req)
}
