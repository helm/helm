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

	"strings"

	"log"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	extensionsclient "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/helm/pkg/chartutil"
)

// Install uses Kubernetes client to install Tiller.
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

// Upgrade uses Kubernetes client to upgrade Tiller to current version.
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
	// If the service does not exists that would mean we are upgrading from a Tiller version
	// that didn't deploy the service, so install it.
	_, err = client.Core().Services(opts.Namespace).Get(serviceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return createService(client.Core(), opts.Namespace)
	}
	return err
}

// createDeployment creates the Tiller Deployment resource.
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

// parseNodeSelectors takes a comma delimited list of key=values pairs and returns a map
func parseNodeSelectors(labels string) map[string]string {
	kv := strings.Split(labels, ",")
	nodeSelectors := map[string]string{}
	nodeSelectors["beta.kubernetes.io/os"] = "linux"
	for _, v := range kv {
		el := strings.Split(v, "=")
		if len(el) == 2 {
			nodeSelectors[el[0]] = el[1]
		}
	}

	return nodeSelectors
}

// istable is a special-purpose function to see if the present thing matches the definition of a YAML table.
func istable(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}
func coalesceTables(dst, src map[string]interface{}) map[string]interface{} {
	// Because dest has higher precedence than src, dest values override src
	// values.
	for key, val := range src {
		if istable(val) {
			if innerdst, ok := dst[key]; !ok {
				dst[key] = val
			} else if istable(innerdst) {
				coalesceTables(innerdst.(map[string]interface{}), val.(map[string]interface{}))
			} else {
				log.Printf("warning: cannot overwrite table with non table for %s (%v)", key, val)
			}
			continue
		} else if dv, ok := dst[key]; ok && istable(dv) {
			log.Printf("warning: destination for %s is a table. Ignoring non-table value %v", key, val)
			continue
		} else if !ok { // <- ok is still in scope from preceding conditional.
			dst[key] = val
			continue
		}
	}
	return dst
}

// pathToMap creates a nested map given a YAML path in dot notation.
func pathToMap(path string, data map[string]interface{}) map[string]interface{} {
	if path == "." {
		return data
	}
	ap := strings.Split(path, ".")
	if len(ap) == 0 {
		return nil
	}
	n := []map[string]interface{}{}
	// created nested map for each key, adding to slice
	for _, v := range ap {
		nm := make(map[string]interface{})
		nm[v] = make(map[string]interface{})
		n = append(n, nm)
	}
	// find the last key (map) and set our data
	for i, d := range n {
		for k := range d {
			z := i + 1
			if z == len(n) {
				n[i][k] = data
				break
			}
			n[i][k] = n[z]
		}
	}

	return n[0]
}

// Merges source and destination map, preferring values from the source map
func mergeValues(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = nextMap
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}

func generateDeployment(opts *Options) *v1beta1.Deployment {
	labels := generateLabels(map[string]string{"name": "tiller"})
	nodeSelectors := map[string]string{}
	if len(opts.NodeSelectors) > 0 {
		nodeSelectors = parseNodeSelectors(opts.NodeSelectors)
	}
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
	// if --set values were specified, ultimately convert values and deployment to maps,
	// merge them and convert back to Deployment
	if len(opts.Values) > 0 {
		// base deployment struct
		var dd v1beta1.Deployment
		// get YAML from original deployment
		dy, err := yaml.Marshal(d)
		if err != nil {
			log.Fatalf("Error marshalling base Tiller Deployment to YAML: %+v", err)
		}
		// convert deployment YAML to values
		dv, err := chartutil.ReadValues(dy)
		if err != nil {
			log.Fatalf("Error converting Deployment manifest to Values: %+v ", err)
		}
		setMap, err := opts.valuesMap()
		// transform our set map back into YAML
		setS, err := yaml.Marshal(setMap)

		if err != nil {
			log.Fatalf("Error marshalling set map to YAML: %+v ", err)
		}
		// transform our YAML into Values
		setV, err := chartutil.ReadValues(setS)

		//log.Fatal(setV)
		if err != nil {
			log.Fatalf("Error reading Values from input: %+v ", err)
		}
		// merge original deployment map and set map
		//finalM := coalesceTables(dv.AsMap(), setV.AsMap())
		//finalM := mergeValues(dv.AsMap(), setV.AsMap())
		dm := dv.AsMap()
		sm := setV.AsMap()
		err = mergo.Merge(&sm, dm)
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal(sm) //for other merges use finalM above
		finalY, err := yaml.Marshal(dm)
		if err != nil {
			log.Fatalf("Error marshalling merged map to YAML: %+v ", err)
		}

		// convert merged values back into deployment
		err = yaml.Unmarshal([]byte(finalY), &dd)
		if err != nil {
			log.Fatalf("Error unmarshalling Values to Deployment manifest: %+v ", err)
		}
		d = &dd
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
