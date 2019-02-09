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

package kube

import (
	"bytes"
	"io"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
)

type mockKubeClient struct{}

func (k *mockKubeClient) Create(ns string, r io.Reader, timeout int64, shouldWait bool) error {
	return nil
}
func (k *mockKubeClient) Get(ns string, r io.Reader) (string, error) {
	return "", nil
}
func (k *mockKubeClient) Delete(ns string, r io.Reader) error {
	return nil
}
func (k *mockKubeClient) Update(ns string, currentReader, modifiedReader io.Reader, force, recreate bool, timeout int64, shouldWait bool) error {
	return nil
}
func (k *mockKubeClient) WatchUntilReady(ns string, r io.Reader, timeout int64, shouldWait bool) error {
	return nil
}
func (k *mockKubeClient) Build(ns string, reader io.Reader) (Result, error) {
	return []*resource.Info{}, nil
}
func (k *mockKubeClient) BuildUnstructured(ns string, reader io.Reader) (Result, error) {
	return []*resource.Info{}, nil
}
func (k *mockKubeClient) WaitAndGetCompletedPodPhase(namespace string, reader io.Reader, timeout time.Duration) (v1.PodPhase, error) {
	return v1.PodUnknown, nil
}

func (k *mockKubeClient) WaitAndGetCompletedPodStatus(namespace string, reader io.Reader, timeout time.Duration) (v1.PodPhase, error) {
	return "", nil
}

var _ KubernetesClient = &mockKubeClient{}
var _ KubernetesClient = &PrintingKubeClient{}

func TestKubeClient(t *testing.T) {
	kc := &mockKubeClient{}

	manifests := map[string]string{
		"foo": "name: value\n",
		"bar": "name: value\n",
	}

	b := bytes.NewBuffer(nil)
	for _, content := range manifests {
		b.WriteString("\n---\n")
		b.WriteString(content)
	}

	if err := kc.Create("sharry-bobbins", b, 300, false); err != nil {
		t.Errorf("Kubeclient failed: %s", err)
	}
}
