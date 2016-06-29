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

package kube

import (
	"fmt"
	"io"
	"log"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/watch"
)

// Client represents a client capable of communicating with the Kubernetes API.
type Client struct {
	*cmdutil.Factory
}

// New create a new Client
func New(config clientcmd.ClientConfig) *Client {
	return &Client{
		Factory: cmdutil.NewFactory(config),
	}
}

// ResourceActorFunc performs an action on a signle resource.
type ResourceActorFunc func(*resource.Info) error

// Create creates kubernetes resources from an io.reader
//
// Namespace will set the namespace
func (c *Client) Create(namespace string, reader io.Reader) error {
	if err := c.ensureNamespace(namespace); err != nil {
		return err
	}
	return perform(c, namespace, reader, createResource)
}

// Delete deletes kubernetes resources from an io.reader
//
// Namespace will set the namespace
func (c *Client) Delete(namespace string, reader io.Reader) error {
	return perform(c, namespace, reader, deleteResource)
}

// WatchUntilReady watches the resource given in the reader, and waits until it is ready.
//
// This function is mainly for hook implementations. It watches for a resource to
// hit a particular milestone. The milestone depends on the Kind.
//
// For most kinds, it checks to see if the resource is marked as Added or Modified
// by the Kubernetes event stream. For some kinds, it does more:
//
// - Jobs: A job is marked "Ready" when it has successfully completed. This is
//   ascertained by watching the Status fields in a job's output.
//
// Handling for other kinds will be added as necessary.
func (c *Client) WatchUntilReady(namespace string, reader io.Reader) error {
	// For jobs, there's also the option to do poll c.Jobs(namespace).Get():
	// https://github.com/adamreese/kubernetes/blob/master/test/e2e/job.go#L291-L300
	return perform(c, namespace, reader, watchUntilReady)
}

const includeThirdPartyAPIs = false

func perform(c *Client, namespace string, reader io.Reader, fn ResourceActorFunc) error {
	r := c.NewBuilder(includeThirdPartyAPIs).
		ContinueOnError().
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(reader, "").
		Flatten().
		Do()

	if r.Err() != nil {
		return r.Err()
	}

	count := 0
	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		err = fn(info)

		if err == nil {
			count++
		}
		return err
	})

	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no objects passed to create")
	}
	return nil
}

func createResource(info *resource.Info) error {
	_, err := resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, true, info.Object)
	return err
}

func deleteResource(info *resource.Info) error {
	return resource.NewHelper(info.Client, info.Mapping).Delete(info.Namespace, info.Name)
}

func watchUntilReady(info *resource.Info) error {
	w, err := resource.NewHelper(info.Client, info.Mapping).WatchSingle(info.Namespace, info.Name, info.ResourceVersion)
	if err != nil {
		return err
	}

	kind := info.Mapping.GroupVersionKind.Kind
	log.Printf("Watching for changes to %s %s", kind, info.Name)
	timeout := time.Minute * 5

	// What we watch for depends on the Kind.
	// - For a Job, we watch for completion.
	// - For all else, we watch until Ready.
	// In the future, we might want to add some special logic for types
	// like Ingress, Volume, etc.

	_, err = watch.Until(timeout, w, func(e watch.Event) (bool, error) {
		switch e.Type {
		case watch.Added, watch.Modified:
			// For things like a secret or a config map, this is the best indicator
			// we get. We care mostly about jobs, where what we want to see is
			// the status go into a good state. For other types, like ReplicaSet
			// we don't really do anything to support these as hooks.
			log.Printf("Add/Modify event for %s: %v", info.Name, e.Type)
			if kind == "Job" {
				return waitForJob(e, info.Name)
			}
			return true, nil
		case watch.Deleted:
			log.Printf("Deleted event for %s", info.Name)
			return true, nil
		case watch.Error:
			// Handle error and return with an error.
			log.Printf("Error event for %s", info.Name)
			return true, fmt.Errorf("Failed to deploy %s", info.Name)
		default:
			return false, nil
		}
	})
	return err
}

// waitForJob is a helper that waits for a job to complete.
//
// This operates on an event returned from a watcher.
func waitForJob(e watch.Event, name string) (bool, error) {
	o, ok := e.Object.(*batch.Job)
	if !ok {
		return true, fmt.Errorf("Expected %s to be a *batch.Job, got %T", name, o)
	}

	for _, c := range o.Status.Conditions {
		if c.Type == batch.JobComplete && c.Status == api.ConditionTrue {
			return true, nil
		} else if c.Type == batch.JobFailed && c.Status == api.ConditionTrue {
			return true, fmt.Errorf("Job failed: %s", c.Reason)
		}
	}

	log.Printf("%s: Jobs active: %d, jobs failed: %d, jobs succeeded: %d", name, o.Status.Active, o.Status.Failed, o.Status.Succeeded)
	return false, nil
}

func (c *Client) ensureNamespace(namespace string) error {
	client, err := c.Client()
	if err != nil {
		return err
	}

	ns := &api.Namespace{}
	ns.Name = namespace
	_, err = client.Namespaces().Create(ns)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
