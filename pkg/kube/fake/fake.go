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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v3/pkg/kube"
)

// FailingKubeClient implements KubeClient for testing purposes. It also has
// additional errors you can set to fail different functions, otherwise it
// delegates all its calls to `PrintingKubeClient`
type FailingKubeClient struct {
	PrintingKubeClient
	CreateError                      error
	GetError                         error
	WaitError                        error
	DeleteError                      error
	DeleteWithPropagationError       error
	WatchUntilReadyError             error
	UpdateError                      error
	UpdateWithTimeoutError           error
	BuildError                       error
	BuildTableError                  error
	BuildDummy                       bool
	BuildUnstructuredError           error
	WaitAndGetCompletedPodPhaseError error
	WaitDuration                     time.Duration
}

// Create returns the configured error if set or prints
func (f *FailingKubeClient) Create(resources kube.ResourceList) (*kube.Result, error) {
	if f.CreateError != nil {
		return nil, f.CreateError
	}
	return f.PrintingKubeClient.Create(resources)
}

// Get returns the configured error if set or prints
func (f *FailingKubeClient) Get(resources kube.ResourceList, related bool) (map[string][]runtime.Object, error) {
	if f.GetError != nil {
		return nil, f.GetError
	}
	return f.PrintingKubeClient.Get(resources, related)
}

// Waits the amount of time defined on f.WaitDuration, then returns the configured error if set or prints.
func (f *FailingKubeClient) Wait(resources kube.ResourceList, d time.Duration) error {
	time.Sleep(f.WaitDuration)
	if f.WaitError != nil {
		return f.WaitError
	}
	return f.PrintingKubeClient.Wait(resources, d)
}

// WaitWithJobs returns the configured error if set or prints
func (f *FailingKubeClient) WaitWithJobs(resources kube.ResourceList, d time.Duration) error {
	if f.WaitError != nil {
		return f.WaitError
	}
	return f.PrintingKubeClient.WaitWithJobs(resources, d)
}

// WaitForDelete returns the configured error if set or prints
func (f *FailingKubeClient) WaitForDelete(resources kube.ResourceList, d time.Duration) error {
	if f.WaitError != nil {
		return f.WaitError
	}
	return f.PrintingKubeClient.WaitForDelete(resources, d)
}

// Delete returns the configured error if set or prints
func (f *FailingKubeClient) Delete(resources kube.ResourceList) (*kube.Result, []error) {
	if f.DeleteError != nil {
		return nil, []error{f.DeleteError}
	}
	return f.PrintingKubeClient.Delete(resources)
}

// WatchUntilReady returns the configured error if set or prints
func (f *FailingKubeClient) WatchUntilReady(resources kube.ResourceList, d time.Duration) error {
	if f.WatchUntilReadyError != nil {
		return f.WatchUntilReadyError
	}
	return f.PrintingKubeClient.WatchUntilReady(resources, d)
}

// Update returns the configured error if set or prints
func (f *FailingKubeClient) Update(r, modified kube.ResourceList, ignoreMe bool) (*kube.Result, error) {
	if f.UpdateError != nil {
		return &kube.Result{}, f.UpdateError
	}
	return f.PrintingKubeClient.Update(r, modified, ignoreMe)
}

func (f *FailingKubeClient) UpdateWithTimeout(r, modified kube.ResourceList, ignoreMe bool, timeout time.Duration) (*kube.Result, error) {
	if f.UpdateWithTimeoutError != nil {
		return &kube.Result{}, f.UpdateWithTimeoutError
	}
	return f.PrintingKubeClient.UpdateWithTimeout(r, modified, ignoreMe, 0)
}

// Build returns the configured error if set or prints
func (f *FailingKubeClient) Build(r io.Reader, _ bool) (kube.ResourceList, error) {
	if f.BuildError != nil {
		return []*resource.Info{}, f.BuildError
	}
	if f.BuildDummy {
		return createDummyResourceList(), nil
	}
	return f.PrintingKubeClient.Build(r, false)
}

// BuildTable returns the configured error if set or prints
func (f *FailingKubeClient) BuildTable(r io.Reader, _ bool) (kube.ResourceList, error) {
	if f.BuildTableError != nil {
		return []*resource.Info{}, f.BuildTableError
	}
	return f.PrintingKubeClient.BuildTable(r, false)
}

// WaitAndGetCompletedPodPhase returns the configured error if set or prints
func (f *FailingKubeClient) WaitAndGetCompletedPodPhase(s string, d time.Duration) (v1.PodPhase, error) {
	if f.WaitAndGetCompletedPodPhaseError != nil {
		return v1.PodSucceeded, f.WaitAndGetCompletedPodPhaseError
	}
	return f.PrintingKubeClient.WaitAndGetCompletedPodPhase(s, d)
}

// DeleteWithPropagationPolicy returns the configured error if set or prints
func (f *FailingKubeClient) DeleteWithPropagationPolicy(resources kube.ResourceList, policy metav1.DeletionPropagation) (*kube.Result, []error) {
	if f.DeleteWithPropagationError != nil {
		return nil, []error{f.DeleteWithPropagationError}
	}
	return f.PrintingKubeClient.DeleteWithPropagationPolicy(resources, policy)
}

func createDummyResourceList() kube.ResourceList {
	var resInfo resource.Info
	resInfo.Name = "dummyName"
	resInfo.Namespace = "dummyNamespace"
	var resourceList kube.ResourceList
	resourceList.Append(&resInfo)
	return resourceList

}
