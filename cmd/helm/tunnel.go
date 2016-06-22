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
