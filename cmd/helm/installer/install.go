/*
Copyright The Helm Authors.

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
	"io/ioutil"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/tiller/environment"
)

// Install uses Kubernetes client to install Tiller.
//
// Returns an error if the command failed.
func Install(client kubernetes.Interface, opts *Options) error {
	if err := createDeployment(client.AppsV1(), opts); err != nil {
		return err
	}
	if err := createService(client.CoreV1(), opts.Namespace); err != nil {
		return err
	}
	if opts.tls() {
		if err := createSecret(client.CoreV1(), opts); err != nil {
			return err
		}
	}
	return nil
}

// Upgrade uses Kubernetes client to upgrade Tiller to current version.
//
// Returns an error if the command failed.
func Upgrade(client kubernetes.Interface, opts *Options) error {
	appsobj, err := client.AppsV1().Deployments(opts.Namespace).Get(deploymentName, metav1.GetOptions{})
	if err == nil {
		// Can happen in two cases:
		// 1. helm init inserted an apps/v1 Deployment up front in Kubernetes
		// 2. helm init inserted an extensions/v1beta1 Deployment against a K8s cluster already
		//    supporting apps/v1 Deployment. In such a case K8s is returning the apps/v1 object anyway.`
		//    (for the same reason "kubectl convert" is being deprecated)
		return upgradeAppsTillerDeployment(client, opts, appsobj)
	}

	extensionsobj, err := client.ExtensionsV1beta1().Deployments(opts.Namespace).Get(deploymentName, metav1.GetOptions{})
	if err == nil {
		// User performed helm init against older version of kubernetes (Previous to 1.9)
		return upgradeExtensionsTillerDeployment(client, opts, extensionsobj)
	}

	return err
}

func upgradeAppsTillerDeployment(client kubernetes.Interface, opts *Options, obj *appsv1.Deployment) error {
	// Update the PodTemplateSpec section of the deployment
	if err := updatePodTemplate(&obj.Spec.Template.Spec, opts); err != nil {
		return err
	}

	if _, err := client.AppsV1().Deployments(opts.Namespace).Update(obj); err != nil {
		return err
	}

	// If the service does not exist that would mean we are upgrading from a Tiller version
	// that didn't deploy the service, so install it.
	_, err := client.CoreV1().Services(opts.Namespace).Get(serviceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return createService(client.CoreV1(), opts.Namespace)
	}

	return err
}

func upgradeExtensionsTillerDeployment(client kubernetes.Interface, opts *Options, obj *extensionsv1beta1.Deployment) error {
	// Update the PodTemplateSpec section of the deployment
	if err := updatePodTemplate(&obj.Spec.Template.Spec, opts); err != nil {
		return err
	}

	if _, err := client.ExtensionsV1beta1().Deployments(opts.Namespace).Update(obj); err != nil {
		return err
	}

	// If the service does not exist that would mean we are upgrading from a Tiller version
	// that didn't deploy the service, so install it.
	_, err := client.CoreV1().Services(opts.Namespace).Get(serviceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return createService(client.CoreV1(), opts.Namespace)
	}

	return err
}

func updatePodTemplate(podSpec *v1.PodSpec, opts *Options) error {
	tillerImage := podSpec.Containers[0].Image
	clientImage := opts.SelectImage()

	if semverCompare(tillerImage, clientImage) == -1 && !opts.ForceUpgrade {
		return fmt.Errorf("current Tiller version %s is newer than client version %s, use --force-upgrade to downgrade", tillerImage, clientImage)
	}
	podSpec.Containers[0].Image = clientImage
	podSpec.Containers[0].ImagePullPolicy = opts.pullPolicy()
	podSpec.ServiceAccountName = opts.ServiceAccount

	return nil
}

// semverCompare returns whether the client's version is older, equal or newer than the given image's version.
func semverCompare(tillerImage, clientImage string) int {
	tillerVersion, err := string2semver(tillerImage)
	if err != nil {
		// same thing with unparsable tiller versions (e.g. canary releases).
		return 1
	}

	// clientVersion, err := semver.NewVersion(currentVersion)
	clientVersion, err := string2semver(clientImage)
	if err != nil {
		// aaaaaand same thing with unparsable helm versions (e.g. canary releases).
		return 1
	}

	return clientVersion.Compare(tillerVersion)
}

func string2semver(image string) (*semver.Version, error) {
	split := strings.Split(image, ":")
	if len(split) < 2 {
		// If we don't know the version, we consider the client version newer.
		return nil, fmt.Errorf("no repository in image %s", image)
	}
	return semver.NewVersion(split[1])
}

// createDeployment creates the Tiller Deployment resource.
func createDeployment(client appsv1client.DeploymentsGetter, opts *Options) error {
	obj, err := generateDeployment(opts)
	if err != nil {
		return err
	}
	_, err = client.Deployments(obj.Namespace).Create(obj)
	return err

}

// Deployment gets a deployment object that can be used to generate a manifest
// as a string. This object should not be submitted directly to the Kubernetes
// api
func Deployment(opts *Options) (*appsv1.Deployment, error) {
	dep, err := generateDeployment(opts)
	if err != nil {
		return nil, err
	}
	dep.TypeMeta = metav1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	}
	return dep, nil
}

// createService creates the Tiller service resource
func createService(client corev1.ServicesGetter, namespace string) error {
	obj := generateService(namespace)
	_, err := client.Services(obj.Namespace).Create(obj)
	return err
}

// Service gets a service object that can be used to generate a manifest as a
// string. This object should not be submitted directly to the Kubernetes api
func Service(namespace string) *v1.Service {
	svc := generateService(namespace)
	svc.TypeMeta = metav1.TypeMeta{
		Kind:       "Service",
		APIVersion: "v1",
	}
	return svc
}

// TillerManifests gets the Deployment, Service, and Secret (if tls-enabled) manifests
func TillerManifests(opts *Options) ([]string, error) {
	dep, err := Deployment(opts)
	if err != nil {
		return []string{}, err
	}

	svc := Service(opts.Namespace)

	objs := []runtime.Object{dep, svc}

	if opts.EnableTLS {
		secret, err := Secret(opts)
		if err != nil {
			return []string{}, err
		}
		objs = append(objs, secret)
	}

	manifests := make([]string, len(objs))
	for i, obj := range objs {
		o, err := yaml.Marshal(obj)
		if err != nil {
			return []string{}, err
		}
		manifests[i] = string(o)
	}

	return manifests, err
}

func generateLabels(labels map[string]string) map[string]string {
	labels["app"] = "helm"
	return labels
}

// parseNodeSelectorsInto parses a comma delimited list of key=values pairs into a map.
func parseNodeSelectorsInto(labels string, m map[string]string) error {
	kv := strings.Split(labels, ",")
	for _, v := range kv {
		el := strings.Split(v, "=")
		if len(el) == 2 {
			m[el[0]] = el[1]
		} else {
			return fmt.Errorf("invalid nodeSelector label: %q", kv)
		}
	}
	return nil
}
func generateDeployment(opts *Options) (*appsv1.Deployment, error) {
	labels := generateLabels(map[string]string{"name": "tiller"})
	nodeSelectors := map[string]string{}
	if len(opts.NodeSelectors) > 0 {
		err := parseNodeSelectorsInto(opts.NodeSelectors, nodeSelectors)
		if err != nil {
			return nil, err
		}
	}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.Namespace,
			Name:      deploymentName,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: opts.getReplicas(),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					ServiceAccountName:           opts.ServiceAccount,
					AutomountServiceAccountToken: &opts.AutoMountServiceAccountToken,
					Containers: []v1.Container{
						{
							Name:            "tiller",
							Image:           opts.SelectImage(),
							ImagePullPolicy: opts.pullPolicy(),
							Ports: []v1.ContainerPort{
								{ContainerPort: environment.DefaultTillerPort, Name: "tiller"},
								{ContainerPort: environment.DefaultTillerProbePort, Name: "http"},
							},
							Env: []v1.EnvVar{
								{Name: "TILLER_NAMESPACE", Value: opts.Namespace},
								{Name: "TILLER_HISTORY_MAX", Value: fmt.Sprintf("%d", opts.MaxHistory)},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/liveness",
										Port: intstr.FromInt(environment.DefaultTillerProbePort),
									},
								},
								InitialDelaySeconds: 1,
								TimeoutSeconds:      1,
							},
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/readiness",
										Port: intstr.FromInt(environment.DefaultTillerProbePort),
									},
								},
								InitialDelaySeconds: 1,
								TimeoutSeconds:      1,
							},
						},
					},
					HostNetwork:  opts.EnableHostNetwork,
					NodeSelector: nodeSelectors,
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
	// if --override values were specified, ultimately convert values and deployment to maps,
	// merge them and convert back to Deployment
	if len(opts.Values) > 0 {
		// base deployment struct
		var dd appsv1.Deployment
		// get YAML from original deployment
		dy, err := yaml.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("Error marshalling base Tiller Deployment: %s", err)
		}
		// convert deployment YAML to values
		dv, err := chartutil.ReadValues(dy)
		if err != nil {
			return nil, fmt.Errorf("Error converting Deployment manifest: %s ", err)
		}
		dm := dv.AsMap()
		// merge --set values into our map
		sm, err := opts.valuesMap(dm)
		if err != nil {
			return nil, fmt.Errorf("Error merging --set values into Deployment manifest")
		}
		finalY, err := yaml.Marshal(sm)
		if err != nil {
			return nil, fmt.Errorf("Error marshalling merged map to YAML: %s ", err)
		}
		// convert merged values back into deployment
		err = yaml.Unmarshal(finalY, &dd)
		if err != nil {
			return nil, fmt.Errorf("Error unmarshalling Values to Deployment manifest: %s ", err)
		}
		d = &dd
	}

	return d, nil
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
					Port:       environment.DefaultTillerPort,
					TargetPort: intstr.FromString("tiller"),
				},
			},
			Selector: labels,
		},
	}
	return s
}

// Secret gets a secret object that can be used to generate a manifest as a
// string. This object should not be submitted directly to the Kubernetes api
func Secret(opts *Options) (*v1.Secret, error) {
	secret, err := generateSecret(opts)
	if err != nil {
		return nil, err
	}

	secret.TypeMeta = metav1.TypeMeta{
		Kind:       "Secret",
		APIVersion: "v1",
	}

	return secret, nil
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
