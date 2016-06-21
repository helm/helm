package main

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"

	"k8s.io/helm/pkg/kube"
)

// TODO refactor out this global var
var tunnel *kube.Tunnel

func newTillerPortForwarder(namespace string) (*kube.Tunnel, error) {
	podName, err := getTillerPodName(namespace)
	if err != nil {
		return nil, err
	}
	// FIXME use a constain that is accessible on init
	const tillerPort = 44134
	return kube.New(nil).ForwardPort(namespace, podName, tillerPort)
}

func getTillerPodName(namespace string) (string, error) {
	client, err := kube.New(nil).Client()
	if err != nil {
		return "", err
	}

	// TODO use a const for labels
	selector := labels.Set{"app": "helm", "name": "tiller"}.AsSelector()
	options := api.ListOptions{LabelSelector: selector}
	pods, err := client.Pods(namespace).List(options)
	if err != nil {
		return "", err
	}
	if len(pods.Items) < 1 {
		return "", fmt.Errorf("I could not find tiller")
	}
	return pods.Items[0].ObjectMeta.GetName(), nil
}
