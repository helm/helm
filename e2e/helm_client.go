/*
Copyright 2017 The Kubernetes Authors All rights reserved.
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

package e2e

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// HelmManager provides functionality to install client/server helm and use it
type HelmManager interface {
	// InstallTiller will bootstrap tiller pod in k8s
	InstallTiller() error
	// DeleteTiller removes tiller pod from k8s
	DeleteTiller(removeHelmHome bool) error
	// Install chart, returns releaseName and error
	Install(chartName string, values map[string]string) (string, error)
	// Status verifies state of installed release
	Status(releaseName string) error
	// Delete release
	Delete(releaseName string) error
	// Upgrade release
	Upgrade(chartName, releaseName string, values map[string]string) error
	// Rollback release
	Rollback(releaseName string, revision int) error
}

// BinaryHelmManager uses helm binary to work with helm server
type BinaryHelmManager struct {
	Clientset         kubernetes.Interface
	Namespace         string
	HelmBin           string
	TillerHost        string
	UseCanary         bool
	UseServiceAccount bool
}

func (m *BinaryHelmManager) InstallTiller() error {
	args := []string{"init"}
	if m.UseCanary {
		args = append(args, "--canary-image")
	}
	if m.UseServiceAccount {
		args = append(args, "--service-account", "tiller")
		if err := m.InstallServiceAccounts(); err != nil {
			return err
		}
	}
	if _, err := m.executeUsingHelm(args...); err != nil {
		return err
	}
	By("Waiting for tiller pod")
	waitTillerPod(m.Clientset, m.Namespace)
	return nil
}

func (m *BinaryHelmManager) DeleteTiller(removeHelmHome bool) error {
	args := []string{"reset", "--force"}
	if removeHelmHome {
		args = append(args, "--remove-helm-home")
	}
	_, err := m.executeUsingHelm(args...)
	return err
}

func (m *BinaryHelmManager) Install(chartName string, values map[string]string) (string, error) {
	stdout, err := m.executeCommandWithValues(chartName, "install", values)
	if err != nil {
		return "", err
	}
	return getNameFromHelmOutput(stdout), nil
}

// Status reports nil if release is considered to be succesfull
func (m *BinaryHelmManager) Status(releaseName string) error {
	stdout, err := m.executeUsingHelm("status", releaseName)
	if err != nil {
		return err
	}
	status := getStatusFromHelmOutput(stdout)
	if status == "DEPLOYED" {
		return nil
	}
	return fmt.Errorf("Expected status is DEPLOYED. But got %v for release %v.", status, releaseName)
}

func (m *BinaryHelmManager) Delete(releaseName string) error {
	_, err := m.executeUsingHelm("delete", releaseName)
	return err
}

func (m *BinaryHelmManager) Upgrade(chartName, releaseName string, values map[string]string) error {
	args := []string{"upgrade", releaseName, chartName}
	if len(values) > 0 {
		args = append(args, "--set", prepareArgsFromValues(values))
	}
	_, err := m.executeUsingHelmInNamespace(args...)
	return err
}

func (m *BinaryHelmManager) Rollback(releaseName string, revision int) error {
	_, err := m.executeUsingHelm("rollback", releaseName, strconv.Itoa(revision))
	return err
}

func (m *BinaryHelmManager) executeUsingHelmInNamespace(args ...string) (string, error) {
	return m.executeUsingHelm(append(args, "--namespace", m.Namespace)...)
}

func (m *BinaryHelmManager) executeUsingHelm(args ...string) (string, error) {
	if m.TillerHost != "" {
		args = append(args, "--host", m.TillerHost)
	}
	return m.executeUsingBinary(m.HelmBin, append(args, "--tiller-namespace", m.Namespace)...)
}

func (m *BinaryHelmManager) executeUsingBinary(binary string, args ...string) (string, error) {
	cmd := exec.Command(binary, args...)
	Logf("Running command %+v\n", cmd.Args)
	stdout, err := cmd.Output()
	if err != nil {
		switch err.(type) {
		case *exec.ExitError:
			stderr := err.(*exec.ExitError)
			Logf("Command %+v, Err %s\n", cmd.Args, stderr.Stderr)
		case *exec.Error:
			Logf("Command %+v, Err %s\n", cmd.Args, err)
		}
		return "", err
	}
	return string(stdout), nil
}

func (m *BinaryHelmManager) executeCommandWithValues(releaseName, command string, values map[string]string) (string, error) {
	args := []string{command, releaseName}
	if len(values) > 0 {
		vals := prepareArgsFromValues(values)
		args = append(args, "--set", vals)
	}
	return m.executeUsingHelmInNamespace(args...)
}

func (m *BinaryHelmManager) InstallServiceAccounts() error {
	objects := strings.Replace(serviceAccountTemplate, "TILLER_NAMESPACE", m.Namespace, -1)

	f, err := ioutil.TempFile("", m.Namespace)
	if err != nil {
		Logf("Failed creating tempfile: %s", err)
		return err
	}

	f.WriteString(objects)
	f.Sync()

	_, err = m.executeUsingBinary("kubectl", "create", "-f", f.Name())
	return err
}

func regexpKeyFromStructuredOutput(key, output string) string {
	r := regexp.MustCompile(fmt.Sprintf("%v:[[:space:]]*(.*)", key))
	// key will be captured in group with index 1
	result := r.FindStringSubmatch(output)
	if len(result) < 2 {
		return ""
	}
	return result[1]
}

func getNameFromHelmOutput(output string) string {
	return regexpKeyFromStructuredOutput("NAME", output)
}

func getStatusFromHelmOutput(output string) string {
	return regexpKeyFromStructuredOutput("STATUS", output)
}

func waitTillerPod(clientset kubernetes.Interface, namespace string) {
	Eventually(func() bool {
		pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		if err != nil {
			return false
		}
		for _, pod := range pods.Items {
			if !strings.Contains(pod.Name, "tiller") {
				continue
			}
			Logf("Found tiller pod. Phase %v\n", pod.Status.Phase)
			if pod.Status.Phase != v1.PodRunning {
				return false
			}
			for _, cond := range pod.Status.Conditions {
				if cond.Type != v1.PodReady {
					continue
				}
				return cond.Status == v1.ConditionTrue
			}
		}
		return false
	}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "tiller pod is not running in namespace "+namespace)
}

func prepareArgsFromValues(values map[string]string) string {
	var b bytes.Buffer
	for key, val := range values {
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(val)
		b.WriteString(",")
	}
	return b.String()
}

var serviceAccountTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tiller
  namespace: TILLER_NAMESPACE
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: tiller-manager
  namespace: TILLER_NAMESPACE
rules:
- apiGroups: ["", "extensions", "apps", "*"]
  resources: ["*"]
  verbs: ["*"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: tiller-binding
  namespace: TILLER_NAMESPACE
subjects:
- kind: ServiceAccount
  name: tiller
  namespace: TILLER_NAMESPACE
roleRef:
  kind: Role
  name: tiller-manager
  apiGroup: rbac.authorization.k8s.io`
