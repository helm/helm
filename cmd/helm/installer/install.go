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
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/util/intstr"

	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/version"
)

const defaultImage = "gcr.io/kubernetes-helm/tiller"

// Install uses kubernetes client to install tiller
//
// Returns the string output received from the operation, and an error if the
// command failed.
//
// If verbose is true, this will print the manifest to stdout.
func Install(namespace, image string, verbose bool) error {
	kc := kube.New(nil)

	if namespace == "" {
		ns, _, err := kc.DefaultNamespace()
		if err != nil {
			return err
		}
		namespace = ns
	}

	c, err := kc.Client()
	if err != nil {
		return err
	}

	ns := generateNamespace(namespace)
	if _, err := c.Namespaces().Create(ns); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}

	if image == "" {
		// strip git sha off version
		tag := strings.Split(version.GetVersion(), "+")[0]
		image = fmt.Sprintf("%s:%s", defaultImage, tag)
	}

	rc := generateDeployment(image)

	_, err = c.Deployments(namespace).Create(rc)
	return err
}

func generateLabels(labels map[string]string) map[string]string {
	labels["app"] = "helm"
	return labels
}

func generateDeployment(image string) *extensions.Deployment {
	labels := generateLabels(map[string]string{"name": "tiller"})
	d := &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name:   "tiller-deploy",
			Labels: labels,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: 1,
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: labels,
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:            "tiller",
							Image:           image,
							ImagePullPolicy: "Always",
							Ports:           []api.ContainerPort{{ContainerPort: 44134, Name: "tiller"}},
							LivenessProbe: &api.Probe{
								Handler: api.Handler{
									HTTPGet: &api.HTTPGetAction{
										Path: "/liveness",
										Port: intstr.FromInt(44135),
									},
								},
								InitialDelaySeconds: 1,
								TimeoutSeconds:      1,
							},
							ReadinessProbe: &api.Probe{
								Handler: api.Handler{
									HTTPGet: &api.HTTPGetAction{
										Path: "/readiness",
										Port: intstr.FromInt(44135),
									},
								},
								InitialDelaySeconds: 1,
								TimeoutSeconds:      1,
							},
						},
					},
				},
			},
		},
	}
	return d
}

func generateNamespace(namespace string) *api.Namespace {
	return &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name:   namespace,
			Labels: generateLabels(map[string]string{"name": "helm-namespace"}),
		},
	}
}
