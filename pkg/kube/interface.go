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
)

// Interface represents a client capable of communicating with the Kubernetes API.
//
// A KubernetesClient must be concurrency safe.
type Interface interface {
	// Create creates one or more resources.
	Create(resources ResourceList) (*Result, error)

	Wait(resources ResourceList, timeout time.Duration) error

	// Delete destroys one or more resources.
	Delete(resources ResourceList) (*Result, []error)

	// Watch the resource in reader until it is "ready". This method
	//
	// For Jobs, "ready" means the Job ran to completion (exited without error).
	// For Pods, "ready" means the Pod phase is marked "succeeded".
	// For all other kinds, it means the kind was created or modified without
	// error.
	WatchUntilReady(resources ResourceList, timeout time.Duration) error

	// Update updates one or more resources or creates the resource
	// if it doesn't exist.
	Update(original, target ResourceList, force bool) (*Result, error)

	// Build creates a resource list from a Reader
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n")
	//
	// Validates against OpenAPI schema if validate is true.
	Build(reader io.Reader, validate bool) (ResourceList, error)

	// WaitAndGetCompletedPodPhase waits up to a timeout until a pod enters a completed phase
	// and returns said phase (PodSucceeded or PodFailed qualify).
	WaitAndGetCompletedPodPhase(name string, timeout time.Duration) (v1.PodPhase, error)

	// isReachable checks whether the client is able to connect to the cluster
	IsReachable() error
}

// Extended Kubernetes client interface
//
// Version 2 interface adds new methods in a backward compatible way.
// In the next API breaking release it could be merged with the base interface.
type InterfaceV2 interface {
	Interface

	// Update updates one or more resources or creates the resource if it doesn't exist.
	//
	// Force controls how to perform the update of a resource:
	//
	// force: false
	//   Patch a resource, if that fails due to an StatusReasonInvalid or StatusReasonConflict error,
	//   delete it and recreate it afterwards.
	// force: true
	//   Delete and recreated without trying to patch it first.
	//
	// After deleting a resource poll and wait until resource was deleted, fails if server does not delete resource within timeout.
	UpdateRecreate(original, target ResourceList, force bool, timeout time.Duration) (*Result, error)
}

var _ InterfaceV2 = (*Client)(nil)
