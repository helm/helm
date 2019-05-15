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

// KubernetesClient represents a client capable of communicating with the Kubernetes API.
//
// A KubernetesClient must be concurrency safe.
type Interface interface {
	// Create creates one or more resources.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Create(reader io.Reader) error

	Wait(r io.Reader, timeout time.Duration) error

	// Delete destroys one or more resources.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Delete(io.Reader) error

	// Watch the resource in reader until it is "ready".
	//
	// For Jobs, "ready" means the job ran to completion (excited without error).
	// For all other kinds, it means the kind was created or modified without
	// error.
	WatchUntilReady(reader io.Reader, timeout time.Duration) error

	// Update updates one or more resources or creates the resource
	// if it doesn't exist.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Update(originalReader, modifiedReader io.Reader, force bool, recreate bool) error

	Build(reader io.Reader) (Result, error)
	BuildUnstructured(reader io.Reader) (Result, error)

	// WaitAndGetCompletedPodPhase waits up to a timeout until a pod enters a completed phase
	// and returns said phase (PodSucceeded or PodFailed qualify).
	WaitAndGetCompletedPodPhase(name string, timeout time.Duration) (v1.PodPhase, error)
}

var _ Interface = (*Client)(nil)
