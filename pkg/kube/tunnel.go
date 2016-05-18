package kube

import (
	"bytes"
	"fmt"
	"net"
	"strconv"

	"k8s.io/kubernetes/pkg/client/unversioned/portforward"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
)

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

	// http://192.168.64.94:8080/api/v1/namespaces/helm/pods/tiller-rc-9itlq/portforward
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

	go func() {
		if err := pf.ForwardPorts(); err != nil {
			fmt.Printf("Error forwarding ports: %v\n", err)
		}
	}()

	// wait for listeners to start
	<-pf.Ready

	return t, nil
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
