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

package environment

import (
	"bytes"
	"io"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

type mockEngine struct {
	out map[string]string
}

func (e *mockEngine) Render(chrt *chart.Chart, v chartutil.Values) (map[string]string, error) {
	return e.out, nil
}

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
func (k *mockKubeClient) Update(ns string, currentReader, modifiedReader io.Reader, force bool, recreate bool, timeout int64, shouldWait bool) error {
	return nil
}
func (k *mockKubeClient) WatchUntilReady(ns string, r io.Reader, timeout int64, shouldWait bool) error {
	return nil
}
func (k *mockKubeClient) Build(ns string, reader io.Reader) (kube.Result, error) {
	return []*resource.Info{}, nil
}
func (k *mockKubeClient) BuildUnstructured(ns string, reader io.Reader) (kube.Result, error) {
	return []*resource.Info{}, nil
}
func (k *mockKubeClient) WaitAndGetCompletedPodPhase(namespace string, reader io.Reader, timeout time.Duration) (core.PodPhase, error) {
	return core.PodUnknown, nil
}

func (k *mockKubeClient) WaitAndGetCompletedPodStatus(namespace string, reader io.Reader, timeout time.Duration) (core.PodPhase, error) {
	return "", nil
}

var _ Engine = &mockEngine{}
var _ KubeClient = &mockKubeClient{}
var _ KubeClient = &PrintingKubeClient{}

func TestEngine(t *testing.T) {
	eng := &mockEngine{out: map[string]string{"albatross": "test"}}

	env := New()
	env.EngineYard = EngineYard(map[string]Engine{"test": eng})

	if engine, ok := env.EngineYard.Get("test"); !ok {
		t.Errorf("failed to get engine from EngineYard")
	} else if out, err := engine.Render(&chart.Chart{}, map[string]interface{}{}); err != nil {
		t.Errorf("unexpected template error: %s", err)
	} else if out["albatross"] != "test" {
		t.Errorf("expected 'test', got %q", out["albatross"])
	}
}

func TestKubeClient(t *testing.T) {
	kc := &mockKubeClient{}
	env := New()
	env.KubeClient = kc

	manifests := map[string]string{
		"foo": "name: value\n",
		"bar": "name: value\n",
	}

	b := bytes.NewBuffer(nil)
	for _, content := range manifests {
		b.WriteString("\n---\n")
		b.WriteString(content)
	}

	if err := env.KubeClient.Create("sharry-bobbins", b, 300, false); err != nil {
		t.Errorf("Kubeclient failed: %s", err)
	}
}
