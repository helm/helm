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

package fake

import (
	"io"
	"runtime"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"

	"helm.sh/helm/v3/pkg/kube"
)

// KubeClient wrapper which can be used for testing to verify that a certain method has been called a certain
// number of times
type KubeClientSpy struct {
	KubeClientV2 kube.InterfaceV2
	// map with function names as keys and number of times it was called as names
	Calls map[string]int
}

func NewKubeClientSpy(kubeClient kube.InterfaceV2) KubeClientSpy {
	return KubeClientSpy{
		KubeClientV2: kubeClient,
		Calls:        make(map[string]int),
	}
}

func functionName() string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	pathSegments := strings.Split(frame.Function, ".")
	return pathSegments[len(pathSegments)-1]
}

func (v KubeClientSpy) Create(resources kube.ResourceList) (*kube.Result, error) {
	v.Calls[functionName()]++
	return v.KubeClientV2.Create(resources)
}

func (v KubeClientSpy) Wait(resources kube.ResourceList, timeout time.Duration) error {
	v.Calls[functionName()]++
	return v.KubeClientV2.Wait(resources, timeout)
}

func (v KubeClientSpy) Delete(resources kube.ResourceList) (*kube.Result, []error) {
	v.Calls[functionName()]++
	return v.KubeClientV2.Delete(resources)
}

func (v KubeClientSpy) WatchUntilReady(resources kube.ResourceList, timeout time.Duration) error {
	v.Calls[functionName()]++
	return v.KubeClientV2.WatchUntilReady(resources, timeout)
}

func (v KubeClientSpy) Update(original, target kube.ResourceList, force bool) (*kube.Result, error) {
	v.Calls[functionName()]++
	return v.KubeClientV2.Update(original, target, force)
}

func (v KubeClientSpy) Build(reader io.Reader, validate bool) (kube.ResourceList, error) {
	v.Calls[functionName()]++
	return v.KubeClientV2.Build(reader, validate)
}

func (v KubeClientSpy) WaitAndGetCompletedPodPhase(name string, timeout time.Duration) (v1.PodPhase, error) {
	v.Calls[functionName()]++
	return v.KubeClientV2.WaitAndGetCompletedPodPhase(name, timeout)
}

func (v KubeClientSpy) IsReachable() error {
	v.Calls[functionName()]++
	return v.KubeClientV2.IsReachable()
}

func (v KubeClientSpy) UpdateRecreate(original, target kube.ResourceList, force bool, timeout time.Duration) (*kube.Result, error) {
	v.Calls[functionName()]++
	return v.KubeClientV2.UpdateRecreate(original, target, force, timeout)
}

func (v KubeClientSpy) WaitWithJobs(resources kube.ResourceList, timeout time.Duration) error {
	v.Calls[functionName()]++
	return v.KubeClientV2.WaitWithJobs(resources, timeout)
}
