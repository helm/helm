/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package kube

import (
	"bytes"
	"fmt"
	"net"
	"strconv"

	"k8s.io/kubernetes/pkg/client/unversioned/portforward"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
)

// Tunnel describes a ssh-like tunnel to a kubernetes pod
type Tunnel struct {
	Local    int
	Remote   int
	stopChan chan struct{}
}

// Close disconnects a tunnel connection
func (t *Tunnel) Close() {
	close(t.stopChan)
}

// ForwardPort opens a tunnel to a kubernetes pod
func (c *Client) ForwardPort(namespace, podName string, remote int) (*Tunnel, error) {
	client, err := c.Client()
	if err != nil {
		return nil, err
	}

	config, err := c.ClientConfig()
	if err != nil {
		return nil, err
	}

	// Build a url to the portforward endpoing
	// example: http://localhost:8080/api/v1/namespaces/helm/pods/tiller-deploy-9itlq/portforward
	u := client.RESTClient.Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").URL()

	dialer, err := remotecommand.NewExecutor(config, "POST", u)
	if err != nil {
		return nil, err
	}

	local, err := getAvailablePort()
	if err != nil {
		return nil, err
	}

	t := &Tunnel{
		Local:    local,
		Remote:   remote,
		stopChan: make(chan struct{}, 1),
	}

	ports := []string{fmt.Sprintf("%d:%d", local, remote)}

	var b bytes.Buffer
	pf, err := portforward.New(dialer, ports, t.stopChan, &b, &b)
	if err != nil {
		return nil, err
	}

	errChan := make(chan error)
	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		return t, fmt.Errorf("Error forwarding ports: %v\n", err)
	case <-pf.Ready:
		return t, nil
	}
}

func getAvailablePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()

	_, p, err := net.SplitHostPort(l.Addr().String())
	port, err := strconv.Atoi(p)
	if err != nil {
		return 0, err
	}
	return port, err
}
