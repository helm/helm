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
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
)

// KubernetesKubectl implements the interface for talking to Kubernetes by wrapping calls
// via kubectl.
type KubernetesKubectl struct {
	KubePath string
	// Base level arguments to kubectl. User commands/arguments get appended to this.
	Arguments []string
}

func NewKubernetesKubectl(config *KubernetesConfig) Kubernetes {
	if config.KubePath == "" {
		log.Fatalf("kubectl path cannot be empty")
	}
	// If a configuration file is specified, then it will provide the server
	// address and credentials. If not, then we check for the server address
	// and credentials as individual flags.
	var args []string
	if config.KubeConfig != "" {
		config.KubeConfig = os.ExpandEnv(config.KubeConfig)
		args = append(args, fmt.Sprintf("--kubeconfig=%s", config.KubeConfig))
	} else {
		if config.KubeServer != "" {
			args = append(args, fmt.Sprintf("--server=https://%s", config.KubeServer))
		} else if config.KubeService != "" {
			addrs, err := net.LookupHost(config.KubeService)
			if err != nil || len(addrs) < 1 {
				log.Fatalf("cannot resolve DNS name: %v", config.KubeService)
			}

			args = append(args, fmt.Sprintf("--server=https://%s", addrs[0]))
		}

		if config.KubeInsecure {
			args = append(args, fmt.Sprintf("--insecure-skip-tls-verify=%s", config.KubeInsecure))
		} else {
			if config.KubeCertAuth != "" {
				args = append(args, fmt.Sprintf("--certificate-authority=%s", config.KubeCertAuth))
				if config.KubeClientCert == "" {
					args = append(args, fmt.Sprintf("--client-certificate=%s", config.KubeClientCert))
				}

				if config.KubeClientKey == "" {
					args = append(args, fmt.Sprintf("--client-key=%s", config.KubeClientKey))
				}
			}
		}
		if config.KubeToken != "" {
			args = append(args, fmt.Sprintf("--token=%s", config.KubeToken))
		} else {
			if config.KubeUsername != "" {
				args = append(args, fmt.Sprintf("--username=%s", config.KubeUsername))
			}

			if config.KubePassword != "" {
				args = append(args, fmt.Sprintf("--password=%s", config.KubePassword))
			}
		}
	}
	return &KubernetesKubectl{config.KubePath, args}
}

func (k *KubernetesKubectl) Get(name string, resourceType string) (string, error) {
	args := []string{"get"}
	// Specify output as json rather than human readable for easier machine parsing
	args = append(args, "-o", "json")
	args = append(args, resourceType)
	args = append(args, name)
	return k.execute(args, "")
}

func (k *KubernetesKubectl) Create(resource string) (string, error) {
	args := []string{"create"}
	//	args = append(args, resourceType)
	//	args = append(args, name)
	return k.execute(args, resource)
}

func (k *KubernetesKubectl) Delete(name string, resourceType string) (string, error) {
	args := []string{"delete"}
	args = append(args, resourceType)
	args = append(args, name)
	return k.execute(args, "")
}

func (k *KubernetesKubectl) execute(args []string, input string) (string, error) {
	if len(input) > 0 {
		args = append(args, "-f", "-")
	}

	// Tack on the common arguments to the end of the command line
	args = append(args, k.Arguments...)
	cmd := exec.Command(k.KubePath, args...)
	cmd.Stdin = bytes.NewBuffer([]byte(input))

	// Combine stdout and stderr into a single dynamically resized buffer
	combined := &bytes.Buffer{}
	cmd.Stdout = combined
	cmd.Stderr = combined

	if err := cmd.Start(); err != nil {
		log.Printf("cannot start kubectl %#v", err)
		return combined.String(), err
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("kubectl failed:  %#v", err)
		return combined.String(), err
	}
	return combined.String(), nil
}
