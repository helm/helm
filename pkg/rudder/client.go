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
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	rudderAPI "k8s.io/helm/pkg/proto/hapi/rudder"
)

// RudderDefaultAddress will be used if none is provided with command lines argumnets.
const (
	RudderDefaultAddress = "127.0.0.1:10001"
)

// NewClient creates new instance of Client.
func NewClient(address string) (*Client, error) {
	//TODO(mkwiek): parametrize this
	conn, err := grpc.Dial(
		address,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithTimeout(3*time.Second))
	if err != nil {
		return nil, fmt.Errorf("error establishing connection with rudder using address %s: %v", address, err)
	}
	return &Client{client: rudderAPI.NewReleaseModuleServiceClient(conn)}, nil
}

// Client wraps rudder grpc client.
type Client struct {
	client rudderAPI.ReleaseModuleServiceClient
}

// InstallRelease calls Rudder InstallRelease method which should create provided release
func (c *Client) InstallRelease(rel *rudderAPI.InstallReleaseRequest) (*rudderAPI.InstallReleaseResponse, error) {
	return c.client.InstallRelease(context.Background(), rel)
}

// UpgradeRelease calls Rudder UpgradeRelease method which should perform update
func (c *Client) UpgradeRelease(req *rudderAPI.UpgradeReleaseRequest) (*rudderAPI.UpgradeReleaseResponse, error) {
	return c.client.UpgradeRelease(context.Background(), req)
}

// RollbackRelease calls Rudder RollbackRelease method which should perform update
func (c *Client) RollbackRelease(req *rudderAPI.RollbackReleaseRequest) (*rudderAPI.RollbackReleaseResponse, error) {
	return c.client.RollbackRelease(context.Background(), req)
}

// ReleaseStatus calls Rudder ReleaseStatus method which should perform update
func (c *Client) ReleaseStatus(req *rudderAPI.ReleaseStatusRequest) (*rudderAPI.ReleaseStatusResponse, error) {
	return c.client.ReleaseStatus(context.Background(), req)
}

// DeleteRelease calls Rudder DeleteRelease method which should uninstall provided release
func (c *Client) DeleteRelease(rel *rudderAPI.DeleteReleaseRequest) (*rudderAPI.DeleteReleaseResponse, error) {
	return c.client.DeleteRelease(context.Background(), rel)
}
