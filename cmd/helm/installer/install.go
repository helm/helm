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
	"k8s.io/kubernetes/pkg/util/intstr"

	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/version"
)

const defaultImage = "gcr.io/kubernetes-helm/tiller"

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
		tag := strings.Split(version.Version, "+")[0]
		image = fmt.Sprintf("%s:%s", defaultImage, tag)
	}

	rc := generateReplicationController(image)

	_, err = c.ReplicationControllers(namespace).Create(rc)
	return err
}

func generateLabeles(labels map[string]string) map[string]string {
	labels["app"] = "helm"
	return labels
}

func generateReplicationController(image string) *api.ReplicationController {
	labels := generateLabeles(map[string]string{"name": "tiller"})

	rc := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name:   "tiller-rc",
			Labels: labels,
		},
		Spec: api.ReplicationControllerSpec{
			Replicas: 1,
			Selector: labels,
			Template: &api.PodTemplateSpec{
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
	return rc
}

func generateNamespace(namespace string) *api.Namespace {
	return &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name:   namespace,
			Labels: generateLabeles(map[string]string{"name": "helm-namespace"}),
		},
	}
}
