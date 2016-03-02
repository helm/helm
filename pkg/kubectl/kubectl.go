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

package kubectl

// Path is the path of the kubectl binary
var Path = "kubectl"

// Runner is an interface to wrap kubectl convenience methods
type Runner interface {
	// ClusterInfo returns Kubernetes cluster info
	ClusterInfo() ([]byte, error)
	// Create uploads a chart to Kubernetes
	Create(stdin []byte) ([]byte, error)
	// Delete removes a chart from Kubernetes.
	Delete(name string, ktype string) ([]byte, error)
	// Get returns Kubernetes resources
	Get(stdin []byte, ns string) ([]byte, error)

	// GetByKind gets an entry by kind, name, and namespace.
	//
	// If name is omitted, all entries of that kind are returned.
	//
	// If NS is omitted, the default NS is assumed.
	GetByKind(kind, name, ns string) (string, error)
}

// RealRunner implements Runner to execute kubectl commands
type RealRunner struct{}

// PrintRunner implements Runner to return a []byte of the command to be executed
type PrintRunner struct{}

// Client stores the instance of Runner
var Client Runner = RealRunner{}
