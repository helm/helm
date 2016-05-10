package main

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/kubernetes/helm/pkg/kube"
)

func newTillerPortForwarder() (*kube.Tunnel, error) {
	podName, err := getTillerPodName("helm")
	if err != nil {
		return nil, err
	}
	return kube.New(nil).ForwardPort("helm", podName, 44134)
}

func getTillerPodName(namespace string) (string, error) {
	client, err := kube.New(nil).Client()
	if err != nil {
		return "", err
	}

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
