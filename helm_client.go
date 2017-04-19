// Copyright 2017 Mirantis
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"fmt"
	"os/exec"
	"regexp"

	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	experimentalTillerImage string = "nebril/tiller-experimental"
	rudderAppcontroller     string = "rudderAppcontroller"
)

// HelmManager provides functionality to install client/server helm and use it
type HelmManager interface {
	// InstallTiller will bootstrap tiller pod in k8s
	InstallTiller() error
	// DeleteTiller removes tiller pod from k8s
	DeleteTiller(removeHelmHome bool) error
	// Install chart, returns releaseName and error
	Install(chartName string) (string, error)
	// Status verifies state of installed release
	Status(releaseName string) error
}

// BinaryHelmManager uses helm binary to work with helm server
type BinaryHelmManager struct {
	Clientset kubernetes.Interface
	Namespace string
	HelmBin   string
}

func (m *BinaryHelmManager) InstallTiller() error {
	arg := make([]string, 0, 5)
	arg = append(arg, "init", "--tiller-namespace", m.Namespace)
	if enableRudder {
		arg = append(arg, "--tiller-image", experimentalTillerImage)
	}
	_, err := m.executeUsingHelm(arg...)
	if err != nil {
		return err
	}
	By("Waiting for tiller pod")
	waitTillerPod(m.Clientset, m.Namespace)
	if enableRudder {
		return prepareRudder(m.Clientset, m.Namespace)
	}
	return nil
}

func (m *BinaryHelmManager) DeleteTiller(removeHelmHome bool) error {
	arg := make([]string, 0, 4)
	arg = append(arg, "reset", "--tiller-namespace", m.Namespace)
	if removeHelmHome {
		arg = append(arg, "--remove-helm-home")
	}
	_, err := m.executeUsingHelm(arg...)
	if err != nil {
		return err
	}
	if enableRudder {
		return deleteRudder(m.Clientset, m.Namespace)
	}
	return nil
}

func (m *BinaryHelmManager) Install(chartName string) (string, error) {
	stdout, err := m.executeUsingHelmInNamespace("install", chartName)
	if err != nil {
		return "", err
	}
	return getNameFromHelmOutput(stdout), nil
}

// Status reports nil if release is considered to be succesfull
func (m *BinaryHelmManager) Status(releaseName string) error {
	stdout, err := m.executeUsingHelmInNamespace("status", releaseName)
	if err != nil {
		return err
	}
	status := getStatusFromHelmOutput(stdout)
	if status == "DEPLOYED" {
		return nil
	}
	return fmt.Errorf("Expected status is DEPLOYED. But got %v for release %v.", status, releaseName)
}

func (m *BinaryHelmManager) executeUsingHelmInNamespace(arg ...string) (string, error) {
	arg = append(arg, "--namespace", m.Namespace, "--tiller-namespace", m.Namespace)
	return m.executeUsingHelm(arg...)
}

func (m *BinaryHelmManager) executeUsingHelm(arg ...string) (string, error) {
	Logf("Running command %+v\n", arg)
	cmd := exec.Command(m.HelmBin, arg...)
	stdout, err := cmd.Output()
	if err != nil {
		stderr := err.(*exec.ExitError)
		Logf("Ccommand %+v, Err %s\n", arg, stderr.Stderr)
		return "", err
	}
	return string(stdout), nil
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

func prepareRudder(clientset kubernetes.Interface, namespace string) error {
	rudder := &v1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: rudderAppcontroller,
		},
		Spec: v1.PodSpec{
			RestartPolicy: "Always",
			Containers: []v1.Container{
				{
					Name:            "rudder-appcontroller",
					Image:           "mirantis/k8s-appcontroller",
					ImagePullPolicy: v1.PullNever,
				},
			},
		},
	}
	_, err := clientset.Core().Pods(namespace).Create(rudder)
	return err
}

func deleteRudder(clientset kubernetes.Interface, namespace string) error {
	return clientset.Core().Pods(namespace).Delete(rudderAppcontroller, nil)
}

func getNameFromHelmOutput(output string) string {
	return regexpKeyFromStructuredOutput("NAME", output)
}

func getStatusFromHelmOutput(output string) string {
	return regexpKeyFromStructuredOutput("STATUS", output)
}

func waitTillerPod(clientset kubernetes.Interface, namespace string) {
	Eventually(func() bool {
		pods, err := clientset.Core().Pods(namespace).List(v1.ListOptions{})
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
