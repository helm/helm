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

package util

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
)

// KubernetesConfig defines the configuration options for talking to Kubernetes master
type KubernetesConfig struct {
	KubePath       string // The path to kubectl binary
	KubeService    string // DNS name of the kubernetes service
	KubeServer     string // The IP address and optional port of the kubernetes master
	KubeInsecure   bool   // Do not check the server's certificate for validity
	KubeConfig     string // Path to a kubeconfig file
	KubeCertAuth   string // Path to a file for the certificate authority
	KubeClientCert string // Path to a client certificate file
	KubeClientKey  string // Path to a client key file
	KubeToken      string // A service account token
	KubeUsername   string // The username to use for basic auth
	KubePassword   string // The password to use for basic auth
}

// Kubernetes defines the interface for talking to Kubernetes. Currently the
// only implementation is through kubectl, but eventually this could be done
// via direct API calls.
type Kubernetes interface {
	Get(name string, resourceType string) (string, error)
	Create(resource string) (string, error)
	Delete(resource string) (string, error)
	Replace(resource string) (string, error)
}

// KubernetesObject represents a native 'bare' Kubernetes object.
type KubernetesObject struct {
	Kind       string                 `json:"kind"`
	APIVersion string                 `json:"apiVersion"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
}

// KubernetesSecret represents a Kubernetes secret
type KubernetesSecret struct {
	Kind       string            `json:"kind"`
	APIVersion string            `json:"apiVersion"`
	Metadata   map[string]string `json:"metadata"`
	Data       map[string]string `json:"data,omitempty"`
}

// GetServiceURL takes a service name, a service port, and a default service URL,
// and returns a URL for accessing the service. It first looks for an environment
// variable set by Kubernetes by transposing the service name. If it can't find
// one, it looks up the service name in DNS. If that doesn't work, it returns the
// default service URL. If that's empty, it returns an HTTP localhost URL for the
// service port. If service port is empty, it panics.
func GetServiceURL(serviceName, servicePort, serviceURL string) (string, error) {
	if serviceName != "" {
		varBase := strings.Replace(serviceName, "-", "_", -1)
		varName := strings.ToUpper(varBase) + "_PORT"
		serviceURL := os.Getenv(varName)
		if serviceURL != "" {
			u, err := url.Parse(serviceURL)
			if err != nil || u.Path != "" || u.Scheme != "tcp" {
				return "", fmt.Errorf("malformed value: %s for envinronment variable: %s", serviceURL, varName)
			}

			u.Scheme = "http"
			return u.String(), nil
		}

		if servicePort != "" {
			addrs, err := net.LookupHost(serviceName)
			if err == nil && len(addrs) > 0 {
				return fmt.Sprintf("http://%s:%s", addrs[0], servicePort), nil
			}
		}
	}

	if serviceURL != "" {
		return serviceURL, nil
	}

	if servicePort != "" {
		serviceURL = fmt.Sprintf("http://localhost:%s", servicePort)
		return serviceURL, nil
	}

	err := fmt.Errorf("cannot resolve service:%v in environment:%v\n", serviceName, os.Environ())
	return "", err
}

// GetServiceURLOrDie calls GetServiceURL and exits if it returns an error.
func GetServiceURLOrDie(serviceName, servicePort, serviceURL string) string {
	URL, err := GetServiceURL(serviceName, servicePort, serviceURL)
	if err != nil {
		log.Fatal(err)
	}

	return URL
}
