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
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	extensionsclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/internalversion"
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
func Install(client internalclientset.Interface, namespace, image string, canary, verbose bool) error {
	if err := createDeployment(client.Extensions(), namespace, image, canary); err != nil {
		return err
	}
	if err := createService(client.Core(), namespace); err != nil {
		return err
	}
	return nil
}

//
// Upgrade uses kubernetes client to upgrade tiller to current version
//
// Returns an error if the command failed.
func Upgrade(client internalclientset.Interface, namespace, image string, canary bool) error {
	obj, err := client.Extensions().Deployments(namespace).Get("tiller-deploy")
	if err != nil {
		return err
	}
	obj.Spec.Template.Spec.Containers[0].Image = selectImage(image, canary)
	if _, err := client.Extensions().Deployments(namespace).Update(obj); err != nil {
		return err
	}
	// If the service does not exists that would mean we are upgrading from a tiller version
	// that didn't deploy the service, so install it.
	if _, err := client.Core().Services(namespace).Get("tiller-deploy"); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		if err := createService(client.Core(), namespace); err != nil {
			return err
		}
	}
	return nil
}

// createDeployment creates the Tiller deployment reource
func createDeployment(client extensionsclient.DeploymentsGetter, namespace, image string, canary bool) error {
	obj := deployment(namespace, image, canary)
	_, err := client.Deployments(obj.Namespace).Create(obj)
	return err
}

// deployment gets the deployment object that installs Tiller.
func deployment(namespace, image string, canary bool) *extensions.Deployment {
	return generateDeployment(namespace, selectImage(image, canary))
}

// createService creates the Tiller service resource
func createService(client internalversion.ServicesGetter, namespace string) error {
	obj := service(namespace)
	_, err := client.Services(obj.Namespace).Create(obj)
	return err
}

// service gets the service object that installs Tiller.
func service(namespace string) *api.Service {
	return generateService(namespace)
}

func selectImage(image string, canary bool) string {
	switch {
	case canary:
		image = defaultImage + ":canary"
	case image == "":
		image = fmt.Sprintf("%s:%s", defaultImage, version.Version)
	}
	return image
}

// DeploymentManifest gets the manifest (as a string) that describes the Tiller Deployment
// resource.
func DeploymentManifest(namespace, image string, canary bool) (string, error) {
	obj := deployment(namespace, image, canary)

	buf, err := yaml.Marshal(obj)
	return string(buf), err
}

// ServiceManifest gets the manifest (as a string) that describes the Tiller Service
// resource.
func ServiceManifest(namespace string) (string, error) {
	obj := service(namespace)

	buf, err := yaml.Marshal(obj)
	return string(buf), err
}

func generateLabels(labels map[string]string) map[string]string {
	labels["app"] = "helm"
	return labels
}

func generateDeployment(namespace, image string) *extensions.Deployment {
	labels := generateLabels(map[string]string{"name": "tiller"})
	d := &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Namespace: namespace,
			Name:      "tiller-deploy",
			Labels:    labels,
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
							ImagePullPolicy: "IfNotPresent",
							Ports: []api.ContainerPort{
								{ContainerPort: 44134, Name: "tiller"},
							},
							Env: []api.EnvVar{
								{Name: "TILLER_NAMESPACE", Value: namespace},
							},
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

func generateService(namespace string) *api.Service {
	labels := generateLabels(map[string]string{"name": "tiller"})
	s := &api.Service{
		ObjectMeta: api.ObjectMeta{
			Namespace: namespace,
			Name:      "tiller-deploy",
			Labels:    labels,
		},
		Spec: api.ServiceSpec{
			Type: api.ServiceTypeClusterIP,
			Ports: []api.ServicePort{
				{
					Name:       "tiller",
					Port:       44134,
					TargetPort: intstr.FromString("tiller"),
				},
			},
			Selector: labels,
		},
	}
	return s
}
