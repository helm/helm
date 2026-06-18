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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/v4/pkg/kube"
)

// FailingKubeClient implements KubeClient for testing purposes. It also has
// additional errors you can set to fail different functions, otherwise it
// delegates all its calls to `PrintingKubeClient`
type FailingKubeClient struct {
	PrintingKubeClient
	CreateError            error
	GetError               error
	DeleteError            error
	UpdateError            error
	BuildError             error
	BuildTableError        error
	ConnectionError        error
	BuildDummy             bool
	DummyResources         kube.ResourceList
	BuildUnstructuredError error
	WaitError              error
	WaitForDeleteError     error
	WatchUntilReadyError   error
	WaitDuration           time.Duration
	// RecordedWaitOptions stores the WaitOptions passed to GetWaiter for testing
	RecordedWaitOptions []kube.WaitOption
}

var _ kube.Interface = &FailingKubeClient{}

// FailingKubeWaiter implements kube.Waiter for testing purposes.
// It also has additional errors you can set to fail different functions, otherwise it delegates all its calls to `PrintingKubeWaiter`
type FailingKubeWaiter struct {
	*PrintingKubeWaiter
	waitError            error
	waitForDeleteError   error
	watchUntilReadyError error
	waitDuration         time.Duration
}

// Create returns the configured error if set or prints
func (f *FailingKubeClient) Create(resources kube.ResourceList, options ...kube.ClientCreateOption) (*kube.Result, error) {
	if f.CreateError != nil {
		return nil, f.CreateError
	}
	return f.PrintingKubeClient.Create(resources, options...)
}

// Get returns the configured error if set or prints
func (f *FailingKubeClient) Get(resources kube.ResourceList, related bool) (map[string][]runtime.Object, error) {
	if f.GetError != nil {
		return nil, f.GetError
	}
	return f.PrintingKubeClient.Get(resources, related)
}

// Waits the amount of time defined on f.WaitDuration, then returns the configured error if set or prints.
func (f *FailingKubeWaiter) Wait(resources kube.ResourceList, d time.Duration) error {
	time.Sleep(f.waitDuration)
	if f.waitError != nil {
		return f.waitError
	}
	return f.PrintingKubeWaiter.Wait(resources, d)
}

// WaitWithJobs returns the configured error if set or prints
func (f *FailingKubeWaiter) WaitWithJobs(resources kube.ResourceList, d time.Duration) error {
	if f.waitError != nil {
		return f.waitError
	}
	return f.PrintingKubeWaiter.WaitWithJobs(resources, d)
}

// WaitForDelete returns the configured error if set or prints
func (f *FailingKubeWaiter) WaitForDelete(resources kube.ResourceList, d time.Duration) error {
	if f.waitForDeleteError != nil {
		return f.waitForDeleteError
	}
	return f.PrintingKubeWaiter.WaitForDelete(resources, d)
}

// Delete returns the configured error if set or prints
func (f *FailingKubeClient) Delete(resources kube.ResourceList, deletionPropagation metav1.DeletionPropagation) (*kube.Result, []error) {
	if f.DeleteError != nil {
		return nil, []error{f.DeleteError}
	}

	return f.PrintingKubeClient.Delete(resources, deletionPropagation)
}

// WatchUntilReady returns the configured error if set or prints
func (f *FailingKubeWaiter) WatchUntilReady(resources kube.ResourceList, d time.Duration) error {
	if f.watchUntilReadyError != nil {
		return f.watchUntilReadyError
	}
	return f.PrintingKubeWaiter.WatchUntilReady(resources, d)
}

// Update returns the configured error if set or prints
func (f *FailingKubeClient) Update(r, modified kube.ResourceList, options ...kube.ClientUpdateOption) (*kube.Result, error) {
	if f.UpdateError != nil {
		return &kube.Result{}, f.UpdateError
	}
	return f.PrintingKubeClient.Update(r, modified, options...)
}

// Build returns the configured error if set or prints
func (f *FailingKubeClient) Build(r io.Reader, _ bool) (kube.ResourceList, error) {
	if f.BuildError != nil {
		return []*resource.Info{}, f.BuildError
	}
	if f.DummyResources != nil {
		return f.DummyResources, nil
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
	if f.BuildDummy {
		return createDummyResourceList(), nil
	}
	return f.PrintingKubeClient.BuildTable(r, false)
}

func (f *FailingKubeClient) GetWaiter(ws kube.WaitStrategy) (kube.Waiter, error) {
	return f.GetWaiterWithOptions(ws)
}

func (f *FailingKubeClient) GetWaiterWithOptions(ws kube.WaitStrategy, opts ...kube.WaitOption) (kube.Waiter, error) {
	// Record the WaitOptions for testing
	f.RecordedWaitOptions = append(f.RecordedWaitOptions, opts...)
	waiter, _ := f.PrintingKubeClient.GetWaiterWithOptions(ws, opts...)
	printingKubeWaiter, _ := waiter.(*PrintingKubeWaiter)
	return &FailingKubeWaiter{
		PrintingKubeWaiter:   printingKubeWaiter,
		waitError:            f.WaitError,
		waitForDeleteError:   f.WaitForDeleteError,
		watchUntilReadyError: f.WatchUntilReadyError,
		waitDuration:         f.WaitDuration,
	}, nil
}

func (f *FailingKubeClient) IsReachable() error {
	if f.ConnectionError != nil {
		return f.ConnectionError
	}
	return f.PrintingKubeClient.IsReachable()
}

func createDummyResourceList() kube.ResourceList {
	var resInfo resource.Info
	resInfo.Name = "dummyName"
	resInfo.Namespace = "dummyNamespace"
	var resourceList kube.ResourceList
	resourceList.Append(&resInfo)
	return resourceList
}
