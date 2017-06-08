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
	"io/ioutil"

	"github.com/ghodss/yaml"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	extensionsclient "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

// Install uses kubernetes client to install tiller.
//
// Returns an error if the command failed.
func Install(client kubernetes.Interface, opts *Options) error {
	if err := createDeployment(client.Extensions(), opts); err != nil {
		return err
	}
	if err := createService(client.Core(), opts.Namespace); err != nil {
		return err
	}
	if opts.tls() {
		if err := createSecret(client.Core(), opts); err != nil {
			return err
		}
	}
	return nil
}

// Upgrade uses kubernetes client to upgrade tiller to current version.
//
// Returns an error if the command failed.
func Upgrade(client kubernetes.Interface, opts *Options) error {
	obj, err := client.Extensions().Deployments(opts.Namespace).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	obj.Spec.Template.Spec.Containers[0].Image = opts.selectImage()
	obj.Spec.Template.Spec.Containers[0].ImagePullPolicy = opts.pullPolicy()
	obj.Spec.Template.Spec.ServiceAccountName = opts.ServiceAccount
	if _, err := client.Extensions().Deployments(opts.Namespace).Update(obj); err != nil {
		return err
	}
	// If the service does not exists that would mean we are upgrading from a tiller version
	// that didn't deploy the service, so install it.
	_, err = client.Core().Services(opts.Namespace).Get(serviceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return createService(client.Core(), opts.Namespace)
	}
	return err
}

// createDeployment creates the Tiller deployment reource
func createDeployment(client extensionsclient.DeploymentsGetter, opts *Options) error {
	obj := deployment(opts)
	_, err := client.Deployments(obj.Namespace).Create(obj)
	return err
}

// deployment gets the deployment object that installs Tiller.
func deployment(opts *Options) *v1beta1.Deployment {
	return generateDeployment(opts)
}

// createService creates the Tiller service resource
func createService(client corev1.ServicesGetter, namespace string) error {
	obj := service(namespace)
	_, err := client.Services(obj.Namespace).Create(obj)
	return err
}

// service gets the service object that installs Tiller.
func service(namespace string) *v1.Service {
	return generateService(namespace)
}

// DeploymentManifest gets the manifest (as a string) that describes the Tiller Deployment
// resource.
func DeploymentManifest(opts *Options) (string, error) {
	obj := deployment(opts)
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

func generateDeployment(opts *Options) *v1beta1.Deployment {
	labels := generateLabels(map[string]string{"name": "tiller"})
	d := &v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.Namespace,
			Name:      deploymentName,
			Labels:    labels,
		},
		Spec: v1beta1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					ServiceAccountName: opts.ServiceAccount,
					Containers: []v1.Container{
						{
							Name:            "tiller",
							Image:           opts.selectImage(),
							ImagePullPolicy: opts.pullPolicy(),
							Ports: []v1.ContainerPort{
								{ContainerPort: 44134, Name: "tiller"},
							},
							Env: []v1.EnvVar{
								{Name: "TILLER_NAMESPACE", Value: opts.Namespace},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/liveness",
										Port: intstr.FromInt(44135),
									},
								},
								InitialDelaySeconds: 1,
								TimeoutSeconds:      1,
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/readiness",
										Port: intstr.FromInt(44135),
									},
								},
								InitialDelaySeconds: 1,
								TimeoutSeconds:      1,
							},
						},
					},
					HostNetwork: opts.EnableHostNetwork,
					NodeSelector: map[string]string{
						"beta.kubernetes.io/os": "linux",
					},
				},
			},
		},
	}

	if opts.tls() {
		const certsDir = "/etc/certs"

		var tlsVerify, tlsEnable = "", "1"
		if opts.VerifyTLS {
			tlsVerify = "1"
		}

		// Mount secret to "/etc/certs"
		d.Spec.Template.Spec.Containers[0].VolumeMounts = append(d.Spec.Template.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      "tiller-certs",
			ReadOnly:  true,
			MountPath: certsDir,
		})
		// Add environment variable required for enabling TLS
		d.Spec.Template.Spec.Containers[0].Env = append(d.Spec.Template.Spec.Containers[0].Env, []v1.EnvVar{
			{Name: "TILLER_TLS_VERIFY", Value: tlsVerify},
			{Name: "TILLER_TLS_ENABLE", Value: tlsEnable},
			{Name: "TILLER_TLS_CERTS", Value: certsDir},
		}...)
		// Add secret volume to deployment
		d.Spec.Template.Spec.Volumes = append(d.Spec.Template.Spec.Volumes, v1.Volume{
			Name: "tiller-certs",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "tiller-secret",
				},
			},
		})
	}
	return d
}

func generateService(namespace string) *v1.Service {
	labels := generateLabels(map[string]string{"name": "tiller"})
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      serviceName,
			Labels:    labels,
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
			Ports: []v1.ServicePort{
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

// SecretManifest gets the manifest (as a string) that describes the Tiller Secret resource.
func SecretManifest(opts *Options) (string, error) {
	o, err := generateSecret(opts)
	if err != nil {
		return "", err
	}
	buf, err := yaml.Marshal(o)
	return string(buf), err
}

// createSecret creates the Tiller secret resource.
func createSecret(client corev1.SecretsGetter, opts *Options) error {
	o, err := generateSecret(opts)
	if err != nil {
		return err
	}
	_, err = client.Secrets(o.Namespace).Create(o)
	return err
}

// generateSecret builds the secret object that hold Tiller secrets.
func generateSecret(opts *Options) (*v1.Secret, error) {
	const secretName = "tiller-secret"

	labels := generateLabels(map[string]string{"name": "tiller"})
	secret := &v1.Secret{
		Type: v1.SecretTypeOpaque,
		Data: make(map[string][]byte),
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Labels:    labels,
			Namespace: opts.Namespace,
		},
	}
	var err error
	if secret.Data["tls.key"], err = read(opts.TLSKeyFile); err != nil {
		return nil, err
	}
	if secret.Data["tls.crt"], err = read(opts.TLSCertFile); err != nil {
		return nil, err
	}
	if opts.VerifyTLS {
		if secret.Data["ca.crt"], err = read(opts.TLSCaCertFile); err != nil {
			return nil, err
		}
	}
	return secret, nil
}

func read(path string) (b []byte, err error) { return ioutil.ReadFile(path) }
