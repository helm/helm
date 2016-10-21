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

	"github.com/ghodss/yaml"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/intstr"

	"k8s.io/helm/pkg/version"
)

const defaultImage = "gcr.io/kubernetes-helm/tiller"

// Install uses kubernetes client to install tiller
//
// Returns the string output received from the operation, and an error if the
// command failed.
//
// If verbose is true, this will print the manifest to stdout.
func Install(client unversioned.DeploymentsNamespacer, namespace, image string, canary, verbose bool) error {
	obj := deployment(image, canary)
	_, err := client.Deployments(namespace).Create(obj)
	return err
}

// deployment gets the deployment object that installs Tiller.
func deployment(image string, canary bool) *extensions.Deployment {
	switch {
	case canary:
		image = defaultImage + ":canary"
	case image == "":
		image = fmt.Sprintf("%s:%s", defaultImage, version.Version)
	}
	return generateDeployment(image)
}

// DeploymentManifest gets the manifest (as a string) that describes the Tiller Deployment
// resource.
func DeploymentManifest(image string, canary bool) (string, error) {
	obj := deployment(image, canary)

	buf, err := yaml.Marshal(obj)
	return string(buf), err
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
