/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package repo

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/ghodss/yaml"
	"github.com/kubernetes/helm/pkg/util"
)

var (
	kubePath       = flag.String("kubectl", "./kubectl", "The path to the kubectl binary.")
	kubeService    = flag.String("service", "", "The DNS name of the kubernetes service.")
	kubeServer     = flag.String("server", "", "The IP address and optional port of the kubernetes master.")
	kubeInsecure   = flag.Bool("insecure-skip-tls-verify", false, "Do not check the server's certificate for validity.")
	kubeConfig     = flag.String("config", "", "Path to a kubeconfig file.")
	kubeCertAuth   = flag.String("certificate-authority", "", "Path to a file for the certificate authority.")
	kubeClientCert = flag.String("client-certificate", "", "Path to a client certificate file.")
	kubeClientKey  = flag.String("client-key", "", "Path to a client key file.")
	kubeToken      = flag.String("token", "", "A service account token.")
	kubeUsername   = flag.String("username", "", "The username to use for basic auth.")
	kubePassword   = flag.String("password", "", "The password to use for basic auth.")
)

var kubernetesConfig *util.KubernetesConfig

const secretType = "Secret"

// SecretsCredentialProvider provides credentials for registries from Kubernertes secrets.
type SecretsCredentialProvider struct {
	// Actual object that talks to secrets service.
	k util.Kubernetes
}

// NewSecretsCredentialProvider creates a new secrets credential provider.
func NewSecretsCredentialProvider() ICredentialProvider {
	kubernetesConfig := &util.KubernetesConfig{
		KubePath:       *kubePath,
		KubeService:    *kubeService,
		KubeServer:     *kubeServer,
		KubeInsecure:   *kubeInsecure,
		KubeConfig:     *kubeConfig,
		KubeCertAuth:   *kubeCertAuth,
		KubeClientCert: *kubeClientCert,
		KubeClientKey:  *kubeClientKey,
		KubeToken:      *kubeToken,
		KubeUsername:   *kubeUsername,
		KubePassword:   *kubePassword,
	}
	return &SecretsCredentialProvider{util.NewKubernetesKubectl(kubernetesConfig)}
}

func parseCredential(credential string) (*Credential, error) {
	var c util.KubernetesSecret
	if err := json.Unmarshal([]byte(credential), &c); err != nil {
		return nil, fmt.Errorf("cannot unmarshal credential (%s): %s", credential, err)
	}

	d, err := base64.StdEncoding.DecodeString(c.Data["credential"])
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal credential (%s): %s", c, err)
	}

	// And then finally unmarshal it from yaml to Credential
	r := &Credential{}
	if err := yaml.Unmarshal(d, &r); err != nil {
		return nil, fmt.Errorf("cannot unmarshal credential %s (%#v)", c, err)
	}

	return r, nil
}

// GetCredential returns a credential by name.
func (scp *SecretsCredentialProvider) GetCredential(name string) (*Credential, error) {
	o, err := scp.k.Get(name, secretType)
	if err != nil {
		return nil, err
	}

	return parseCredential(o)
}

// SetCredential sets a credential by name.
func (scp *SecretsCredentialProvider) SetCredential(name string, credential *Credential) error {
	// Marshal the credential & base64 encode it.
	b, err := yaml.Marshal(credential)
	if err != nil {
		log.Printf("yaml marshal failed for credential named %s: %s", name, err)
		return err
	}

	enc := base64.StdEncoding.EncodeToString(b)

	// Then create a kubernetes object out of it
	metadata := make(map[string]string)
	metadata["name"] = name
	data := make(map[string]string)
	data["credential"] = enc
	obj := &util.KubernetesSecret{
		Kind:       secretType,
		APIVersion: "v1",
		Metadata:   metadata,
		Data:       data,
	}

	ko, err := yaml.Marshal(obj)
	if err != nil {
		log.Printf("yaml marshal failed for kubernetes object named %s: %s", name, err)
		return err
	}

	_, err = scp.k.Create(string(ko))
	return err
}
