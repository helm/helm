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
	"io"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
)

// KubernetesClient represents a client capable of communicating with the Kubernetes API.
//
// A KubernetesClient must be concurrency safe.
type KubernetesClient interface {
	// Create creates one or more resources.
	//
	// namespace must contain a valid existing namespace.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Create(namespace string, reader io.Reader, timeout int64, shouldWait bool) error

	// Get gets one or more resources. Returned string hsa the format like kubectl
	// provides with the column headers separating the resource types.
	//
	// namespace must contain a valid existing namespace.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Get(namespace string, reader io.Reader) (string, error)

	// Delete destroys one or more resources.
	//
	// namespace must contain a valid existing namespace.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Delete(namespace string, reader io.Reader) error

	// Watch the resource in reader until it is "ready".
	//
	// For Jobs, "ready" means the job ran to completion (excited without error).
	// For all other kinds, it means the kind was created or modified without
	// error.
	WatchUntilReady(namespace string, reader io.Reader, timeout int64, shouldWait bool) error

	// Update updates one or more resources or creates the resource
	// if it doesn't exist.
	//
	// namespace must contain a valid existing namespace.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Update(namespace string, originalReader, modifiedReader io.Reader, force bool, recreate bool, timeout int64, shouldWait bool) error

	Build(namespace string, reader io.Reader) (Result, error)
	BuildUnstructured(namespace string, reader io.Reader) (Result, error)

	// WaitAndGetCompletedPodPhase waits up to a timeout until a pod enters a completed phase
	// and returns said phase (PodSucceeded or PodFailed qualify).
	WaitAndGetCompletedPodPhase(namespace string, reader io.Reader, timeout time.Duration) (v1.PodPhase, error)
}

// PrintingKubeClient implements KubeClient, but simply prints the reader to
// the given output.
type PrintingKubeClient struct {
	Out io.Writer
}

// Create prints the values of what would be created with a real KubeClient.
func (p *PrintingKubeClient) Create(ns string, r io.Reader, timeout int64, shouldWait bool) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// Get prints the values of what would be created with a real KubeClient.
func (p *PrintingKubeClient) Get(ns string, r io.Reader) (string, error) {
	_, err := io.Copy(p.Out, r)
	return "", err
}

// Delete implements KubeClient delete.
//
// It only prints out the content to be deleted.
func (p *PrintingKubeClient) Delete(ns string, r io.Reader) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// WatchUntilReady implements KubeClient WatchUntilReady.
func (p *PrintingKubeClient) WatchUntilReady(ns string, r io.Reader, timeout int64, shouldWait bool) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// Update implements KubeClient Update.
func (p *PrintingKubeClient) Update(ns string, currentReader, modifiedReader io.Reader, force, recreate bool, timeout int64, shouldWait bool) error {
	_, err := io.Copy(p.Out, modifiedReader)
	return err
}

// Build implements KubeClient Build.
func (p *PrintingKubeClient) Build(ns string, reader io.Reader) (Result, error) {
	return []*resource.Info{}, nil
}

// BuildUnstructured implements KubeClient BuildUnstructured.
func (p *PrintingKubeClient) BuildUnstructured(ns string, reader io.Reader) (Result, error) {
	return []*resource.Info{}, nil
}

// WaitAndGetCompletedPodPhase implements KubeClient WaitAndGetCompletedPodPhase.
func (p *PrintingKubeClient) WaitAndGetCompletedPodPhase(namespace string, reader io.Reader, timeout time.Duration) (v1.PodPhase, error) {
	_, err := io.Copy(p.Out, reader)
	return v1.PodUnknown, err
}
