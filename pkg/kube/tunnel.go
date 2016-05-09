package kube

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/portforward"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
)

type tunnel interface {
	Close()
}

type Tunnel struct {
	name   string
	stopCh chan struct{}
}

func (t *Tunnel) Close() {
	close(t.stopCh)
}

func newTunnel(c *Client, namespace, podName string) (*Tunnel, error) {
	client, err := c.factory().Client()
	if err != nil {
		return nil, err
	}

	if err := isPodRunning(client, namespace, podName); err != nil {
		return nil, err
	}

	config, err := c.factory().ClientConfig()
	if err != nil {
		return nil, err
	}

	req := client.RESTClient.Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	stopCh := make(chan struct{}, 1)

	ports := []string{"44134"}

	dialer, err := remotecommand.NewExecutor(config, "POST", req.URL())
	if err != nil {
		return nil, err
	}

	fw, err := portforward.New(dialer, ports, stopCh)
	if err != nil {
		return nil, err
	}

	go func() {
		err = fw.ForwardPorts()
		if err != nil {
			fmt.Printf("Failed to forward ports %v on pod %s: %v\n", ports, podName, err)
		}
	}()

	return &Tunnel{stopCh: stopCh}, nil
}

func isPodRunning(client *unversioned.Client, namespace, podName string) error {
	pod, err := client.Pods(namespace).Get(podName)
	if err != nil {
		return err
	}

	if pod.Status.Phase != api.PodRunning {
		return fmt.Errorf("Unable to execute command because pod is not running. Current status=%v", pod.Status.Phase)
	}
	return nil
}

type portForwarder interface {
	ForwardPorts(url *url.URL, config *restclient.Config, ports []string, stopChan <-chan struct{}) error
}

type defaultPortForwarder struct{}

func (f *defaultPortForwarder) ForwardPorts(url *url.URL, config *restclient.Config, ports []string, stopChan <-chan struct{}) error {
	dialer, err := remotecommand.NewExecutor(config, "POST", url)
	if err != nil {
		return err
	}
	fw, err := portforward.New(dialer, ports, stopChan)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func RunPortForward(f *cmdutil.Factory, namespace, podName string, args []string, fw portForwarder) error {

	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := isPodRunning(client, namespace, podName); err != nil {
		return nil, err
	}

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	stopCh := make(chan struct{}, 1)
	go func() {
		<-signals
		close(stopCh)
	}()

	req := client.RESTClient.Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod.Name).
		SubResource("portforward")

	return fw.ForwardPorts(req.URL(), config, args, stopCh)
}
