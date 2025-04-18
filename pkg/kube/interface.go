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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Interface represents a client capable of communicating with the Kubernetes API.
//
// A KubernetesClient must be concurrency safe.
type Interface interface {
	// Create creates one or more resources.
	Create(resources ResourceList) (*Result, error)

	// Wait waits up to the given timeout for the specified resources to be ready.
	Wait(resources ResourceList, timeout time.Duration) error

	// WaitWithJobs wait up to the given timeout for the specified resources to be ready, including jobs.
	WaitWithJobs(resources ResourceList, timeout time.Duration) error

	// Delete destroys one or more resources.
	Delete(resources ResourceList) (*Result, []error)

	// WatchUntilReady watches the resources given and waits until it is ready.
	//
	// This method is mainly for hook implementations. It watches for a resource to
	// hit a particular milestone. The milestone depends on the Kind.
	//
	// For Jobs, "ready" means the Job ran to completion (exited without error).
	// For Pods, "ready" means the Pod phase is marked "succeeded".
	// For all other kinds, it means the kind was created or modified without
	// error.
	WatchUntilReady(resources ResourceList, timeout time.Duration) error

	// Update updates one or more resources or creates the resource
	// if it doesn't exist.
	Update(original, target ResourceList, force bool) (*Result, error)

	// Build creates a resource list from a Reader.
	//
	// Reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n")
	//
	// Validates against OpenAPI schema if validate is true.
	Build(reader io.Reader, validate bool) (ResourceList, error)

	// WaitAndGetCompletedPodPhase waits up to a timeout until a pod enters a completed phase
	// and returns said phase (PodSucceeded or PodFailed qualify).
	WaitAndGetCompletedPodPhase(name string, timeout time.Duration) (v1.PodPhase, error)

	// IsReachable checks whether the client is able to connect to the cluster.
	IsReachable() error
}

// InterfaceExt was introduced to avoid breaking backwards compatibility for Interface implementers.
//
// TODO Helm 4: Remove InterfaceExt and integrate its method(s) into the Interface.
type InterfaceExt interface {
	// WaitForDelete wait up to the given timeout for the specified resources to be deleted.
	WaitForDelete(resources ResourceList, timeout time.Duration) error
}

// InterfaceThreeWayMerge was introduced to avoid breaking backwards compatibility for Interface implementers.
//
// TODO Helm 4: Remove InterfaceThreeWayMerge and integrate its method(s) into the Interface.
type InterfaceThreeWayMerge interface {
	UpdateThreeWayMerge(original, target ResourceList, force bool) (*Result, error)
}

// InterfaceLogs was introduced to avoid breaking backwards compatibility for Interface implementers.
//
// TODO Helm 4: Remove InterfaceLogs and integrate its method(s) into the Interface.
type InterfaceLogs interface {
	// GetPodList list all pods that match the specified listOptions
	GetPodList(namespace string, listOptions metav1.ListOptions) (*v1.PodList, error)

	// OutputContainerLogsForPodList output the logs for a pod list
	OutputContainerLogsForPodList(podList *v1.PodList, namespace string, writerFunc func(namespace, pod, container string) io.Writer) error
}

// InterfaceDeletionPropagation is introduced to avoid breaking backwards compatibility for Interface implementers.
//
// TODO Helm 4: Remove InterfaceDeletionPropagation and integrate its method(s) into the Interface.
type InterfaceDeletionPropagation interface {
	// DeleteWithPropagationPolicy destroys one or more resources. The deletion propagation is handled as per the given deletion propagation value.
	DeleteWithPropagationPolicy(resources ResourceList, policy metav1.DeletionPropagation) (*Result, []error)
}

// InterfaceResources is introduced to avoid breaking backwards compatibility for Interface implementers.
//
// TODO Helm 4: Remove InterfaceResources and integrate its method(s) into the Interface.
type InterfaceResources interface {
	// Get details of deployed resources.
	// The first argument is a list of resources to get. The second argument
	// specifies if related pods should be fetched. For example, the pods being
	// managed by a deployment.
	Get(resources ResourceList, related bool) (map[string][]runtime.Object, error)

	// BuildTable creates a resource list from a Reader. This differs from
	// Interface.Build() in that a table kind is returned. A table is useful
	// if you want to use a printer to display the information.
	//
	// Reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n")
	//
	// Validates against OpenAPI schema if validate is true.
	// TODO Helm 4: Integrate into Build with an argument
	BuildTable(reader io.Reader, validate bool) (ResourceList, error)
}

var _ Interface = (*Client)(nil)
var _ InterfaceExt = (*Client)(nil)
var _ InterfaceThreeWayMerge = (*Client)(nil)
var _ InterfaceLogs = (*Client)(nil)
var _ InterfaceDeletionPropagation = (*Client)(nil)
var _ InterfaceResources = (*Client)(nil)
