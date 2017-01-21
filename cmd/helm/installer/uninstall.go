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

package installer // import "k8s.io/helm/cmd/helm/installer"

import (
	"strings"

	"github.com/ghodss/yaml"

	"k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	"k8s.io/helm/pkg/kube"
)

// Uninstall uses kubernetes client to uninstall tiller
func Uninstall(kubeClient internalclientset.Interface, kubeCmd *kube.Client, namespace string, verbose bool) error {
	if _, err := kubeClient.Core().Services(namespace).Get("tiller-deploy"); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
	} else if err := deleteService(kubeClient.Core(), namespace); err != nil {
		return err
	}
	if obj, err := kubeClient.Extensions().Deployments(namespace).Get("tiller-deploy"); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
	} else if err := deleteDeployment(kubeCmd, namespace, obj); err != nil {
		return err
	}
	return nil
}

// deleteService deletes the Tiller Service resource
func deleteService(client internalversion.ServicesGetter, namespace string) error {
	return client.Services(namespace).Delete("tiller-deploy", &api.DeleteOptions{})
}

// deleteDeployment deletes the Tiller Deployment resource
// We need to use the kubeCmd reaper instead of the kube API because GC for deployment dependents
// is not yet supported at the k8s server level (<= 1.5)
func deleteDeployment(kubeCmd *kube.Client, namespace string, obj *extensions.Deployment) error {
	obj.Kind = "Deployment"
	obj.APIVersion = "extensions/v1beta1"
	buf, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	reader := strings.NewReader(string(buf))
	infos, err := kubeCmd.Build(namespace, reader)
	if err != nil {
		return err
	}
	for _, info := range infos {
		reaper, err := kubeCmd.Reaper(info.Mapping)
		if err != nil {
			return err
		}
		err = reaper.Stop(info.Namespace, info.Name, 0, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
