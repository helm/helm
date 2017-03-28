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

package kube // import "k8s.io/helm/pkg/kube"

import (
	log "github.com/Sirupsen/logrus"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

func createNamespace(client internalclientset.Interface, namespace string) error {
	logger.WithFields(log.Fields{
		"_module":   "namespace",
		"_context":  "createNamespace",
		"namespace": namespace,
	}).Debug("Creating new namespace")
	ns := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := client.Core().Namespaces().Create(ns)
	return err
}

func getNamespace(client internalclientset.Interface, namespace string) (*api.Namespace, error) {
	return client.Core().Namespaces().Get(namespace)
}

func ensureNamespace(client internalclientset.Interface, namespace string) error {
	logger.WithFields(log.Fields{
		"_module":   "namespace",
		"_context":  "ensureNamespace",
		"namespace": namespace,
	}).Debug("Ensuring that namespace exists")
	_, err := getNamespace(client, namespace)
	if err != nil && errors.IsNotFound(err) {
		logger.WithFields(log.Fields{
			"_module":   "namespace",
			"_context":  "ensureNamespace",
			"namespace": namespace,
		}).Debug("Namespace does not exist, creating")
		return createNamespace(client, namespace)
	}
	return err
}
