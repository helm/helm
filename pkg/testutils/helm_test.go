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

package testutils

import (
	"context"
	"os"
	"os/exec"
	"testing"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func RunHelmCommand(t *testing.T, args []string) (string, error) {
	t.Helper()

	kubeConfig := os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		t.Fatal("KUBECONFIG environment variable is not set")
	}

	cmd := exec.Command("helm", args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeConfig)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Helm command failed: %s", string(output))
		return string(output), err
	}

	return string(output), nil
}

// Test for `helm upgrade --install --take-ownership`
func TestTakeOwnershipFlag(t *testing.T) {
	ns, cleanup := SetupTestEnvironment(t)
	defer cleanup()
	client := fake.NewSimpleClientset()
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-pod",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Helm",
					Name:       "test-release",
					UID:        "456",
				},
			},
		},
	}

	_, err := client.CoreV1().Pods(ns).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}
	retrievedPod, err := client.CoreV1().Pods(ns).Get(context.Background(), "example-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to retrieve pod: %v", err)
	}

	if len(retrievedPod.OwnerReferences) != 1 || retrievedPod.OwnerReferences[0].Kind != "Helm" {
		t.Errorf("expected owner kind to be Helm, got %v", retrievedPod.OwnerReferences[0].Kind)
	}
}
