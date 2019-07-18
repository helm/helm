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

// Package fake implements various fake KubeClients for use in testing
package fake

import (
	"io"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/pkg/kube"
)

// FailingKubeClient implements KubeClient for testing purposes. It also has
// additional errors you can set to fail different functions, otherwise it
// delegates all its calls to `PrintingKubeClient`
type FailingKubeClient struct {
	PrintingKubeClient
	CreateError error
	WaitError error
	GetError error
	DeleteError error
	WatchUntilReadyError error
	UpdateError error
	BuildError error
	BuildUnstructuredError error
	WaitAndGetCompletedPodPhaseError error
}

// Create returns the configured error if set or prints
func (f *FailingKubeClient) Create(r io.Reader) error {
	if f.CreateError != nil {
		return f.CreateError
	}
	return f.PrintingKubeClient.Create(r)
}

// Wait returns the configured error if set or prints
func (f *FailingKubeClient) Wait(r io.Reader, d time.Duration) error {
	if f.WaitError != nil {
		return f.WaitError
	}
	return f.PrintingKubeClient.Wait(r, d)
}

// Create returns the configured error if set or prints
func (f *FailingKubeClient) Get(r io.Reader) (string, error) {
	if f.GetError != nil {
		return "", f.GetError
	}
	return f.PrintingKubeClient.Get(r)
}

// Delete returns the configured error if set or prints
func (f *FailingKubeClient) Delete(r io.Reader) error {
	if f.DeleteError != nil {
		return f.DeleteError
	}
	return f.PrintingKubeClient.Delete(r)
}

// WatchUntilReady returns the configured error if set or prints
func (f *FailingKubeClient) WatchUntilReady(r io.Reader, d time.Duration) error {
	if f.WatchUntilReadyError != nil {
		return f.WatchUntilReadyError
	}
	return f.PrintingKubeClient.WatchUntilReady(r, d)
}

// Update returns the configured error if set or prints
func (f *FailingKubeClient) Update(r, modifiedReader io.Reader, not, needed bool) error {
	if f.UpdateError != nil {
		return f.UpdateError
	}
	return f.PrintingKubeClient.Update(r, modifiedReader, not, needed)
}

// Build returns the configured error if set or prints
func (f *FailingKubeClient) Build(r io.Reader) (kube.Result, error) {
	if f.BuildError != nil {
		return []*resource.Info{}, f.BuildError
	}
	return f.PrintingKubeClient.Build(r)
}

// BuildUnstructured returns the configured error if set or prints
func (f *FailingKubeClient) BuildUnstructured(r io.Reader) (kube.Result, error) {
	if f.BuildUnstructuredError != nil {
		return []*resource.Info{}, f.BuildUnstructuredError
	}
	return f.PrintingKubeClient.Build(r)
}

// WaitAndGetCompletedPodPhase returns the configured error if set or prints
func (f *FailingKubeClient) WaitAndGetCompletedPodPhase(s string, d time.Duration) (v1.PodPhase, error) {
	if f.WaitAndGetCompletedPodPhaseError != nil {
		return v1.PodSucceeded, f.WaitAndGetCompletedPodPhaseError
	}
	return f.PrintingKubeClient.WaitAndGetCompletedPodPhase(s, d)
}