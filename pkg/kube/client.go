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

package kube // import "k8s.io/helm/pkg/kube"

import (
	"bytes"
	goerrors "errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/watch"
    "k8s.io/kubernetes/pkg/labels"
    "k8s.io/kubernetes/pkg/fields"
    "k8s.io/kubernetes/pkg/api/v1"
    "k8s.io/kubernetes/pkg/apis/apps/v1alpha1"
    "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
)

// ErrNoObjectsVisited indicates that during a visit operation, no matching objects were found.
var ErrNoObjectsVisited = goerrors.New("no objects visited")

// Client represents a client capable of communicating with the Kubernetes API.
type Client struct {
	*cmdutil.Factory
	// IncludeThirdPartyAPIs indicates whether to load "dynamic" APIs.
	//
	// This requires additional calls to the Kubernetes API server, and these calls
	// are not supported by all versions. Additionally, during testing, initializing
	// a client will still attempt to contact a live server. In these situations,
	// this flag may need to be disabled.
	IncludeThirdPartyAPIs bool
	// Validate idicates whether to load a schema for validation.
	Validate bool
	// SchemaCacheDir is the path for loading cached schema.
	SchemaCacheDir string
}

// New create a new Client
func New(config clientcmd.ClientConfig) *Client {
	return &Client{
		Factory:               cmdutil.NewFactory(config),
		IncludeThirdPartyAPIs: true,
		Validate:              true,
		SchemaCacheDir:        clientcmd.RecommendedSchemaFile,
	}
}

// ResourceActorFunc performs an action on a single resource.
type ResourceActorFunc func(*resource.Info) error

// ErrAlreadyExists can be returned where there are no changes
type ErrAlreadyExists struct {
	errorMsg string
}

func (e ErrAlreadyExists) Error() string {
	return fmt.Sprintf("Looks like there are no changes for %s", e.errorMsg)
}

// APIClient returns a Kubernetes API client.
//
// This is necessary because cmdutil.Client is a field, not a method, which
// means it can't satisfy an interface's method requirement. In order to ensure
// that an implementation of environment.KubeClient can access the raw API client,
// it is necessary to add this method.
func (c *Client) APIClient() (unversioned.Interface, error) {
	return c.Client()
}

// Create creates kubernetes resources from an io.reader
//
// Namespace will set the namespace
func (c *Client) Create(namespace string, reader io.Reader) error {
	if err := c.ensureNamespace(namespace); err != nil {
		return err
	}
	return perform(c, namespace, reader, createResource)
}

func (c *Client) newBuilder(namespace string, reader io.Reader) *resource.Builder {
	schema, err := c.Validator(c.Validate, c.SchemaCacheDir)
	if err != nil {
		log.Printf("warning: failed to load schema: %s", err)
	}
	return c.NewBuilder(c.IncludeThirdPartyAPIs).
		ContinueOnError().
		Schema(schema).
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(reader, "").
		Flatten()
}

// Get gets kubernetes resources as pretty printed string
//
// Namespace will set the namespace
func (c *Client) Get(namespace string, reader io.Reader) (string, error) {
	// Since we don't know what order the objects come in, let's group them by the types, so
	// that when we print them, they come looking good (headers apply to subgroups, etc.)
	objs := make(map[string][]runtime.Object)
	err := perform(c, namespace, reader, func(info *resource.Info) error {
		log.Printf("Doing get for: '%s'", info.Name)
		obj, err := resource.NewHelper(info.Client, info.Mapping).Get(info.Namespace, info.Name, info.Export)
		if err != nil {
			return err
		}
		// We need to grab the ObjectReference so we can correctly group the objects.
		or, err := api.GetReference(obj)
		if err != nil {
			log.Printf("FAILED GetReference for: %#v\n%v", obj, err)
			return err
		}

		// Use APIVersion/Kind as grouping mechanism. I'm not sure if you can have multiple
		// versions per cluster, but this certainly won't hurt anything, so let's be safe.
		objType := or.APIVersion + "/" + or.Kind
		objs[objType] = append(objs[objType], obj)
		return nil
	})

	// Ok, now we have all the objects grouped by types (say, by v1/Pod, v1/Service, etc.), so
	// spin through them and print them. Printer is cool since it prints the header only when
	// an object type changes, so we can just rely on that. Problem is it doesn't seem to keep
	// track of tab widths
	buf := new(bytes.Buffer)
	p := kubectl.NewHumanReadablePrinter(kubectl.PrintOptions{})
	for t, ot := range objs {
		_, err = buf.WriteString("==> " + t + "\n")
		if err != nil {
			return "", err
		}
		for _, o := range ot {
			err = p.PrintObj(o, buf)
			if err != nil {
				log.Printf("failed to print object type '%s', object: '%s' :\n %v", t, o, err)
				return "", err
			}
		}
		_, err := buf.WriteString("\n")
		if err != nil {
			return "", err
		}
	}
	return buf.String(), err
}

// Update reads in the current configuration and a target configuration from io.reader
//  and creates resources that don't already exists, updates resources that have been modified
//  in the target configuration and deletes resources from the current configuration that are
//  not present in the target configuration
//
// Namespace will set the namespaces
func (c *Client) Update(namespace string, currentReader, targetReader io.Reader, restart bool) error {
	currentInfos, err := c.newBuilder(namespace, currentReader).Do().Infos()
	if err != nil {
		return fmt.Errorf("failed decoding reader into objects: %s", err)
	}

	target := c.newBuilder(namespace, targetReader).Do()
	if target.Err() != nil {
		return fmt.Errorf("failed decoding reader into objects: %s", target.Err())
	}

	targetInfos := []*resource.Info{}
	updateErrors := []string{}

	err = target.Visit(func(info *resource.Info, err error) error {
		targetInfos = append(targetInfos, info)
		if err != nil {
			return err
		}

		helper := resource.NewHelper(info.Client, info.Mapping)
		if _, err := helper.Get(info.Namespace, info.Name, info.Export); err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("Could not get information about the resource: err: %s", err)
			}

			// Since the resource does not exist, create it.
			if err := createResource(info); err != nil {
				return fmt.Errorf("failed to create resource: %s", err)
			}

			kind := info.Mapping.GroupVersionKind.Kind
			log.Printf("Created a new %s called %s\n", kind, info.Name)
			return nil
		}

		currentObj, err := getCurrentObject(info, currentInfos)
		if err != nil {
			return err
		}

		if err := updateResource(c, info, currentObj, restart); err != nil {
			if alreadyExistErr, ok := err.(ErrAlreadyExists); ok {
				log.Printf(alreadyExistErr.errorMsg)
			} else {
				log.Printf("error updating the resource %s:\n\t %v", info.Name, err)
				updateErrors = append(updateErrors, err.Error())
			}
		}

		return nil
	})

	if err != nil {
		return err
	} else if len(updateErrors) != 0 {
		return fmt.Errorf(strings.Join(updateErrors, " && "))
	}
	deleteUnwantedResources(currentInfos, targetInfos)
	return nil
}

// Delete deletes kubernetes resources from an io.reader
//
// Namespace will set the namespace
func (c *Client) Delete(namespace string, reader io.Reader) error {
	return perform(c, namespace, reader, func(info *resource.Info) error {
		log.Printf("Starting delete for %s %s", info.Name, info.Mapping.GroupVersionKind.Kind)

		reaper, err := c.Reaper(info.Mapping)
		if err != nil {
			// If there is no reaper for this resources, delete it.
			if kubectl.IsNoSuchReaperError(err) {
				err := resource.NewHelper(info.Client, info.Mapping).Delete(info.Namespace, info.Name)
				return skipIfNotFound(err)
			}

			return err
		}

		log.Printf("Using reaper for deleting %s", info.Name)
		err = reaper.Stop(info.Namespace, info.Name, 0, nil)
		return skipIfNotFound(err)
	})
}

func skipIfNotFound(err error) error {
	if err != nil && errors.IsNotFound(err) {
		log.Printf("%v", err)
		return nil
	}
	return err
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

func perform(c *Client, namespace string, reader io.Reader, fn ResourceActorFunc) error {
	infos, err := c.newBuilder(namespace, reader).Do().Infos()
	switch {
	case err != nil:
		return scrubValidationError(err)
	case len(infos) == 0:
		return ErrNoObjectsVisited
	}
	for _, info := range infos {
		if err := fn(info); err != nil {
			return err
		}
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

func updateResource(c *Client, target *resource.Info, currentObj runtime.Object, restart bool) error {

	encoder := api.Codecs.LegacyCodec(registered.EnabledVersions()...)
	originalSerialization, err := runtime.Encode(encoder, currentObj)
	if err != nil {
		return err
	}

	editedSerialization, err := runtime.Encode(encoder, target.Object)
	if err != nil {
		return err
	}

	originalJS, err := yaml.ToJSON(originalSerialization)
	if err != nil {
		return err
	}

	editedJS, err := yaml.ToJSON(editedSerialization)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(originalJS, editedJS) {
		return ErrAlreadyExists{target.Name}
	}

	patch, err := strategicpatch.CreateStrategicMergePatch(originalJS, editedJS, currentObj)
	if err != nil {
		return err
	}

	// send patch to server
	helper := resource.NewHelper(target.Client, target.Mapping)
	_, err = helper.Patch(target.Namespace, target.Name, api.StrategicMergePatchType, patch)

    if err != nil {
		return err
	}

    if restart {
        kind := target.Mapping.GroupVersionKind.Kind

        client, _ := c.Client()
        switch kind {
        case "ReplicationController":
            rc := currentObj.(*v1.ReplicationController)
            err = restartPods(client, target.Namespace, rc.Spec.Selector)
        case "DaemonSet":
            daemonSet := currentObj.(*v1beta1.DaemonSet)
            err = restartPods(client, target.Namespace, daemonSet.Spec.Selector.MatchLabels)
        case "PetSet":
            petSet := currentObj.(*v1alpha1.PetSet)
            err = restartPods(client, target.Namespace, petSet.Spec.Selector.MatchLabels)
        case "ReplicaSet":
            replicaSet := currentObj.(*v1beta1.ReplicaSet)
            err = restartPods(client, target.Namespace, replicaSet.Spec.Selector.MatchLabels)
        }
    }

	return err
}


func restartPods(client *unversioned.Client, namespace string, selector map[string]string) error {
    pods, err := client.Pods(namespace).List(api.ListOptions{
            FieldSelector: fields.Everything(),
            LabelSelector: labels.Set(selector).AsSelector(),
    })

    if err != nil {
        return err
    }

    // Restart pods
    for _, pod := range pods.Items {
        log.Printf("Restarting pod: %v/%v", pod.Namespace, pod.Name)

        // Delete each pod for get them restarted with changed spec.
        err := client.Pods(pod.Namespace).Delete(pod.Name, &api.DeleteOptions{
            Preconditions: &api.Preconditions{
                UID: &pod.UID,
            },
        })

        if err != nil {
            return err
        }
    }

    return nil
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

func deleteUnwantedResources(currentInfos, targetInfos []*resource.Info) {
	for _, cInfo := range currentInfos {
		if _, ok := findMatchingInfo(cInfo, targetInfos); !ok {
			log.Printf("Deleting %s...", cInfo.Name)
			if err := deleteResource(cInfo); err != nil {
				log.Printf("Failed to delete %s, err: %s", cInfo.Name, err)
			}
		}
	}
}

func getCurrentObject(target *resource.Info, infos []*resource.Info) (runtime.Object, error) {
	if found, ok := findMatchingInfo(target, infos); ok {
		return found.Mapping.ConvertToVersion(found.Object, found.Mapping.GroupVersionKind.GroupVersion())
	}
	return nil, fmt.Errorf("no resource with the name %s found", target.Name)
}

// isMatchingInfo returns true if infos match on Name and Kind.
func isMatchingInfo(a, b *resource.Info) bool {
	return a.Name == b.Name && a.Mapping.GroupVersionKind.Kind == b.Mapping.GroupVersionKind.Kind
}

// findMatchingInfo returns the first object that matches target.
func findMatchingInfo(target *resource.Info, infos []*resource.Info) (*resource.Info, bool) {
	for _, info := range infos {
		if isMatchingInfo(target, info) {
			return info, true
		}
	}
	return nil, false
}

// scrubValidationError removes kubectl info from the message
func scrubValidationError(err error) error {
	const stopValidateMessage = "if you choose to ignore these errors, turn validation off with --validate=false"

	if strings.Contains(err.Error(), stopValidateMessage) {
		return goerrors.New(strings.Replace(err.Error(), "; "+stopValidateMessage, "", -1))
	}
	return err
}
