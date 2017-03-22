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
	"encoding/json"
	goerrors "errors"
	"fmt"
	"io"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/v1"
	apps "k8s.io/kubernetes/pkg/apis/apps/v1beta1"
	batchinternal "k8s.io/kubernetes/pkg/apis/batch"
	batch "k8s.io/kubernetes/pkg/apis/batch/v1"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	conditions "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

// ErrNoObjectsVisited indicates that during a visit operation, no matching objects were found.
var ErrNoObjectsVisited = goerrors.New("no objects visited")

// Client represents a client capable of communicating with the Kubernetes API.
type Client struct {
	cmdutil.Factory
	// SchemaCacheDir is the path for loading cached schema.
	SchemaCacheDir string
}

// New create a new Client
func New(config clientcmd.ClientConfig) *Client {
	return &Client{
		Factory:        cmdutil.NewFactory(config),
		SchemaCacheDir: clientcmd.RecommendedSchemaFile,
	}
}

// ResourceActorFunc performs an action on a single resource.
type ResourceActorFunc func(*resource.Info) error

// Create creates kubernetes resources from an io.reader
//
// Namespace will set the namespace
func (c *Client) Create(namespace string, reader io.Reader, timeout int64, shouldWait bool) error {
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "Create",
		"namespace": namespace,
		"timeout":   timeout,
		"wait":      shouldWait,
	}).Debug("Beginning resource creation")
	client, err := c.ClientSet()
	if err != nil {
		return err
	}
	if err := ensureNamespace(client, namespace); err != nil {
		return err
	}
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "Create",
	}).Debug("Building manifests")
	infos, buildErr := c.BuildUnstructured(namespace, reader)
	if buildErr != nil {
		return buildErr
	}
	logger.WithFields(log.Fields{
		"_module":       "client",
		"_context":      "Create",
		"num-manifests": len(infos),
	}).Debug("Creating manifests")
	if err := perform(c, namespace, infos, createResource); err != nil {
		return err
	}
	if shouldWait {
		return c.waitForResources(time.Duration(timeout)*time.Second, infos)
	}
	return nil
}

func (c *Client) newBuilder(namespace string, reader io.Reader) *resource.Result {
	schema, err := c.Validator(true, c.SchemaCacheDir)
	if err != nil {
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "newBuilder",
			"error":    err,
		}).Error("Failed to load schema")
	}
	return c.NewBuilder().
		ContinueOnError().
		Schema(schema).
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(reader, "").
		Flatten().
		Do()
}

// BuildUnstructured validates for Kubernetes objects and returns unstructured infos.
func (c *Client) BuildUnstructured(namespace string, reader io.Reader) (Result, error) {
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "BuildUnstructured",
	}).Debug("Beginning unstructured object build")
	schema, err := c.Validator(true, c.SchemaCacheDir)
	if err != nil {
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "BuildUnstructured",
			"error":    err,
		}).Error("Failed to load schema")
	}

	mapper, typer, err := c.UnstructuredObject()
	if err != nil {
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "BuildUnstructured",
			"error":    err,
		}).Error("Failed to load mapper")
		return nil, err
	}
	var result Result
	result, err = resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(c.UnstructuredClientForMapping), runtime.UnstructuredJSONScheme).
		ContinueOnError().
		Schema(schema).
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(reader, "").
		Flatten().
		Do().Infos()
	return result, scrubValidationError(err)
}

// Build validates for Kubernetes objects and returns resource Infos from a io.Reader.
func (c *Client) Build(namespace string, reader io.Reader) (Result, error) {
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "Build",
	}).Debug("Beginning structured object build")
	var result Result
	result, err := c.newBuilder(namespace, reader).Infos()
	return result, scrubValidationError(err)
}

// Get gets kubernetes resources as pretty printed string
//
// Namespace will set the namespace
func (c *Client) Get(namespace string, reader io.Reader) (string, error) {
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "Get",
		"namespace": namespace,
	}).Debug("Beginning resource get")
	// Since we don't know what order the objects come in, let's group them by the types, so
	// that when we print them, they come looking good (headers apply to subgroups, etc.)
	objs := make(map[string][]runtime.Object)
	infos, err := c.BuildUnstructured(namespace, reader)
	if err != nil {
		return "", err
	}
	err = perform(c, namespace, infos, func(info *resource.Info) error {
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "Get",
			"name":     info.Name,
		}).Info("Getting object")
		obj, err := resource.NewHelper(info.Client, info.Mapping).Get(info.Namespace, info.Name, info.Export)
		if err != nil {
			return err
		}
		// We need to grab the ObjectReference so we can correctly group the objects.
		or, err := api.GetReference(obj)
		if err != nil {
			logger.WithFields(log.Fields{
				"_module":  "client",
				"_context": "Get",
				"error":    err,
				"object":   fmt.Sprintf("%#v", obj),
			}).Error("FAILED GetReference")
			return err
		}

		// Use APIVersion/Kind as grouping mechanism. I'm not sure if you can have multiple
		// versions per cluster, but this certainly won't hurt anything, so let's be safe.
		objType := or.APIVersion + "/" + or.Kind
		objs[objType] = append(objs[objType], obj)
		return nil
	})
	if err != nil {
		return "", err
	}

	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "Get",
	}).Debug("Grouping returned resources")
	// Ok, now we have all the objects grouped by types (say, by v1/Pod, v1/Service, etc.), so
	// spin through them and print them. Printer is cool since it prints the header only when
	// an object type changes, so we can just rely on that. Problem is it doesn't seem to keep
	// track of tab widths
	buf := new(bytes.Buffer)
	p := kubectl.NewHumanReadablePrinter(kubectl.PrintOptions{})
	for t, ot := range objs {
		if _, err = buf.WriteString("==> " + t + "\n"); err != nil {
			return "", err
		}
		for _, o := range ot {
			if err := p.PrintObj(o, buf); err != nil {
				logger.WithFields(log.Fields{
					"_module":  "client",
					"_context": "Get",
					"type":     t,
					"object":   o,
				}).Error("Failed to print")
				return "", err
			}
		}
		if _, err := buf.WriteString("\n"); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

// Update reads in the current configuration and a target configuration from io.reader
//  and creates resources that don't already exists, updates resources that have been modified
//  in the target configuration and deletes resources from the current configuration that are
//  not present in the target configuration
//
// Namespace will set the namespaces
func (c *Client) Update(namespace string, originalReader, targetReader io.Reader, recreate bool, timeout int64, shouldWait bool) error {
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "Update",
		"namespace": namespace,
		"timeout":   timeout,
		"recreate":  recreate,
		"wait":      shouldWait,
	}).Debug("Beginning resource update")
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "Update",
	}).Debug("Rebuilding original objects")
	original, err := c.BuildUnstructured(namespace, originalReader)
	if err != nil {
		return fmt.Errorf("failed decoding reader into objects: %s", err)
	}
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "Update",
	}).Debug("Building new objects")
	target, err := c.BuildUnstructured(namespace, targetReader)
	if err != nil {
		return fmt.Errorf("failed decoding reader into objects: %s", err)
	}

	updateErrors := []string{}
	logger.WithFields(log.Fields{
		"_module":     "client",
		"_context":    "Update",
		"num-objects": len(target),
	}).Debug("Visiting new objects for update")
	err = target.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		kind := info.Mapping.GroupVersionKind.Kind
		helper := resource.NewHelper(info.Client, info.Mapping)
		logger.WithFields(log.Fields{
			"_module":   "client",
			"_context":  "Update",
			"name":      info.Name,
			"kind":      kind,
			"namespace": info.Namespace,
		}).Debug("Attempting to get resource")
		if _, err := helper.Get(info.Namespace, info.Name, info.Export); err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("Could not get information about the resource: err: %s", err)
			}

			// Since the resource does not exist, create it.
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "Update",
				"name":      info.Name,
				"kind":      kind,
				"namespace": info.Namespace,
			}).Debug("Resource did not exist, attempting to create")
			if err := createResource(info); err != nil {
				return fmt.Errorf("failed to create resource: %s", err)
			}
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "Update",
				"name":      info.Name,
				"kind":      kind,
				"namespace": info.Namespace,
			}).Info("Created new resource")
			return nil
		}
		logger.WithFields(log.Fields{
			"_module":   "client",
			"_context":  "Update",
			"name":      info.Name,
			"kind":      kind,
			"namespace": info.Namespace,
		}).Debug("Getting original resource")
		originalInfo := original.Get(info)
		if originalInfo == nil {
			return fmt.Errorf("no resource with the name %s found", info.Name)
		}
		logger.WithFields(log.Fields{
			"_module":   "client",
			"_context":  "Update",
			"name":      info.Name,
			"kind":      kind,
			"namespace": info.Namespace,
		}).Debug("Beginning resource update")
		if err := updateResource(c, info, originalInfo.Object, recreate); err != nil {
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "Update",
				"name":      info.Name,
				"kind":      kind,
				"namespace": info.Namespace,
				"error":     err,
			}).Error("Error updating resource")
			log.Printf("error updating the resource %s:\n\t %v", info.Name, err)
			updateErrors = append(updateErrors, err.Error())
		}

		return nil
	})

	switch {
	case err != nil:
		return err
	case len(updateErrors) != 0:
		return fmt.Errorf(strings.Join(updateErrors, " && "))
	}

	for _, info := range original.Difference(target) {
		logger.WithFields(log.Fields{
			"_module":   "client",
			"_context":  "Update",
			"name":      info.Name,
			"kind":      info.Mapping.GroupVersionKind.Kind,
			"namespace": info.Namespace,
		}).Debug("Deleting resource that no longer in manifest")
		log.Printf("Deleting %s in %s...", info.Name, info.Namespace)
		if err := deleteResource(c, info); err != nil {
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "Update",
				"name":      info.Name,
				"kind":      info.Mapping.GroupVersionKind.Kind,
				"namespace": info.Namespace,
			}).Error("Failed to delete resource")
			log.Printf("Failed to delete %s, err: %s", info.Name, err)
		}
	}
	if shouldWait {
		return c.waitForResources(time.Duration(timeout)*time.Second, target)
	}
	return nil
}

// Delete deletes kubernetes resources from an io.reader
//
// Namespace will set the namespace
func (c *Client) Delete(namespace string, reader io.Reader) error {
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "Delete",
		"namespace": namespace,
	}).Debug("Beginning resource delete")
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "Delete",
		"namespace": namespace,
	}).Debug("Rebuilding objects from manifest")
	infos, err := c.BuildUnstructured(namespace, reader)
	if err != nil {
		return err
	}
	return perform(c, namespace, infos, func(info *resource.Info) error {
		logger.WithFields(log.Fields{
			"_module":   "client",
			"_context":  "Delete",
			"namespace": namespace,
			"name":      info.Name,
			"kind":      info.Mapping.GroupVersionKind.Kind,
		}).Info("Starting delete")
		err := deleteResource(c, info)
		return skipIfNotFound(err)
	})
}

func skipIfNotFound(err error) error {
	if errors.IsNotFound(err) {
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "skipIfNotFound",
			"error":    err,
		}).Info("Skipping not found object")
		return nil
	}
	return err
}

func watchTimeout(t time.Duration) ResourceActorFunc {
	return func(info *resource.Info) error {
		return watchUntilReady(t, info)
	}
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
func (c *Client) WatchUntilReady(namespace string, reader io.Reader, timeout int64, shouldWait bool) error {
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "WatchUntilReady",
		"namespace": namespace,
		"timeout":   timeout,
	}).Debug("Beginning watch of Jobs")
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "WatchUntilReady",
		"namespace": namespace,
	}).Debug("Building resources")
	infos, err := c.Build(namespace, reader)
	if err != nil {
		return err
	}
	// For jobs, there's also the option to do poll c.Jobs(namespace).Get():
	// https://github.com/adamreese/kubernetes/blob/master/test/e2e/job.go#L291-L300
	return perform(c, namespace, infos, watchTimeout(time.Duration(timeout)*time.Second))
}

func perform(c *Client, namespace string, infos Result, fn ResourceActorFunc) error {
	if len(infos) == 0 {
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
	obj, err := resource.NewHelper(info.Client, info.Mapping).Create(info.Namespace, true, info.Object)
	if err != nil {
		return err
	}
	return info.Refresh(obj, true)
}

func deleteResource(c *Client, info *resource.Info) error {
	reaper, err := c.Reaper(info.Mapping)
	if err != nil {
		// If there is no reaper for this resources, delete it.
		if kubectl.IsNoSuchReaperError(err) {
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "deleteResource",
				"name":      info.Name,
				"namespace": info.Namespace,
			}).Info("Resource does not have reaper. Deleting")
			return resource.NewHelper(info.Client, info.Mapping).Delete(info.Namespace, info.Name)
		}
		return err
	}
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "deleteResource",
		"name":      info.Name,
		"namespace": info.Namespace,
	}).Info("Using reaper for delete")
	return reaper.Stop(info.Namespace, info.Name, 0, nil)
}

func createPatch(mapping *meta.RESTMapping, target, current runtime.Object) ([]byte, api.PatchType, error) {
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "createPatch",
	}).Debug("Creating patch")
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "createPatch",
		"object":   current,
	}).Debug("Marshalling old object for merge")
	oldData, err := json.Marshal(current)
	if err != nil {
		return nil, api.StrategicMergePatchType, fmt.Errorf("serializing current configuration: %s", err)
	}
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "createPatch",
		"object":   target,
	}).Debug("Marshalling new object for merge")
	newData, err := json.Marshal(target)
	if err != nil {
		return nil, api.StrategicMergePatchType, fmt.Errorf("serializing target configuration: %s", err)
	}

	if api.Semantic.DeepEqual(oldData, newData) {
		return nil, api.StrategicMergePatchType, nil
	}

	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "createPatch",
	}).Debug("Attempting to get versioned object")
	// Get a versioned object
	versionedObject, err := api.Scheme.New(mapping.GroupVersionKind)
	switch {
	case runtime.IsNotRegisteredError(err):
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "createPatch",
			"kind":     mapping.GroupVersionKind,
		}).Debug("Unregistered kind found. This is likely a third party resource. Falling back to JSON merge patch")
		// fall back to generic JSON merge patch
		patch, err := jsonpatch.CreateMergePatch(oldData, newData)
		return patch, api.MergePatchType, err
	case err != nil:
		return nil, api.StrategicMergePatchType, fmt.Errorf("failed to get versionedObject: %s", err)
	default:
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "createPatch",
			"kind":     mapping.GroupVersionKind,
		}).Info("generating strategic merge patch")
		patch, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, versionedObject)
		return patch, api.StrategicMergePatchType, err
	}
}

func updateResource(c *Client, target *resource.Info, currentObj runtime.Object, recreate bool) error {
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "updateResource",
		"name":      target.Name,
		"namespace": target.Namespace,
	}).Debug("Updating resource")
	patch, patchType, err := createPatch(target.Mapping, target.Object, currentObj)
	if err != nil {
		return fmt.Errorf("failed to create patch: %s", err)
	}
	if patch == nil {
		logger.WithFields(log.Fields{
			"_module":   "client",
			"_context":  "updateResource",
			"name":      target.Name,
			"namespace": target.Namespace,
		}).Info("Looks like there are no changes to the resource")
		// This needs to happen to make sure that tiller has the latest info from the API
		// Otherwise there will be no labels and other functions that use labels will panic
		if err := target.Get(); err != nil {
			return fmt.Errorf("error trying to refresh resource information: %v", err)
		}
		return nil
	}

	logger.WithFields(log.Fields{
		"_module":    "client",
		"_context":   "updateResource",
		"name":       target.Name,
		"namespace":  target.Namespace,
		"patch-type": patchType,
	}).Debug("Sending patch to server")
	// send patch to server
	helper := resource.NewHelper(target.Client, target.Mapping)
	obj, err := helper.Patch(target.Namespace, target.Name, patchType, patch)
	if err != nil {
		return err
	}

	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "updateResource",
		"name":      target.Name,
		"namespace": target.Namespace,
	}).Debug("Refreshing object")
	target.Refresh(obj, true)

	if !recreate {
		return nil
	}

	versioned, err := c.AsVersionedObject(target.Object)
	if runtime.IsNotRegisteredError(err) {
		return nil
	}
	if err != nil {
		return err
	}

	selector, err := getSelectorFromObject(versioned)
	if err != nil {
		return nil
	}
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "updateResource",
		"name":      target.Name,
		"namespace": target.Namespace,
	}).Debug("Recreating pods for object")
	client, _ := c.ClientSet()
	return recreatePods(client, target.Namespace, selector)
}

func getSelectorFromObject(obj runtime.Object) (map[string]string, error) {
	switch typed := obj.(type) {
	case *v1.ReplicationController:
		return typed.Spec.Selector, nil
	case *extensions.ReplicaSet:
		return typed.Spec.Selector.MatchLabels, nil
	case *extensions.Deployment:
		return typed.Spec.Selector.MatchLabels, nil
	case *extensions.DaemonSet:
		return typed.Spec.Selector.MatchLabels, nil
	case *batch.Job:
		return typed.Spec.Selector.MatchLabels, nil
	case *apps.StatefulSet:
		return typed.Spec.Selector.MatchLabels, nil
	default:
		return nil, fmt.Errorf("Unsupported kind when getting selector: %v", obj)
	}
}

func recreatePods(client *internalclientset.Clientset, namespace string, selector map[string]string) error {
	logger.WithFields(log.Fields{
		"_module":  "client",
		"_context": "recreatePods",
		"selector": selector,
	}).Debug("Using selector to recreate pods")
	pods, err := client.Pods(namespace).List(api.ListOptions{
		FieldSelector: fields.Everything(),
		LabelSelector: labels.Set(selector).AsSelector(),
	})
	if err != nil {
		return err
	}

	// Restart pods
	for _, pod := range pods.Items {
		logger.WithFields(log.Fields{
			"_module":   "client",
			"_context":  "recreatePods",
			"namespace": pod.Namespace,
			"name":      pod.Name,
		}).Info("Restarting pod")

		// Delete each pod for get them restarted with changed spec.
		if err := client.Pods(pod.Namespace).Delete(pod.Name, api.NewPreconditionDeleteOptions(string(pod.UID))); err != nil {
			return err
		}
	}
	return nil
}

func watchUntilReady(timeout time.Duration, info *resource.Info) error {
	w, err := resource.NewHelper(info.Client, info.Mapping).WatchSingle(info.Namespace, info.Name, info.ResourceVersion)
	if err != nil {
		return err
	}

	kind := info.Mapping.GroupVersionKind.Kind
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "watchUntilReady",
		"namespace": info.Namespace,
		"name":      info.Name,
		"kind":      kind,
		"timeout":   timeout,
	}).Info("Watching for changes")

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
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "watchUntilReady",
				"namespace": info.Namespace,
				"name":      info.Name,
				"kind":      kind,
				"type":      e.Type,
			}).Info("Add/Modify event on watched resource")
			if kind == "Job" {
				return waitForJob(e, info.Name)
			}
			return true, nil
		case watch.Deleted:
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "watchUntilReady",
				"namespace": info.Namespace,
				"name":      info.Name,
				"kind":      kind,
			}).Info("Deleted event on watched resource")
			return true, nil
		case watch.Error:
			// Handle error and return with an error.
			logger.WithFields(log.Fields{
				"_module":   "client",
				"_context":  "watchUntilReady",
				"namespace": info.Namespace,
				"name":      info.Name,
				"kind":      kind,
			}).Error("Error event on watched resource")
			return true, fmt.Errorf("Failed to deploy %s", info.Name)
		default:
			return false, nil
		}
	})
	return err
}

func podsReady(pods []api.Pod) bool {
	for _, pod := range pods {
		if !api.IsPodReady(&pod) {
			return false
		}
	}
	return true
}

func servicesReady(svc []api.Service) bool {
	for _, s := range svc {
		// Make sure the service is not explicitly set to "None" before checking the IP
		if s.Spec.ClusterIP != api.ClusterIPNone && !api.IsServiceIPSet(&s) {
			return false
		}
		// This checks if the service has a LoadBalancer and that balancer has an Ingress defined
		if s.Spec.Type == api.ServiceTypeLoadBalancer && s.Status.LoadBalancer.Ingress == nil {
			return false
		}
	}
	return true
}

func volumesReady(vols []api.PersistentVolumeClaim) bool {
	for _, v := range vols {
		if v.Status.Phase != api.ClaimBound {
			return false
		}
	}
	return true
}

func getPods(client *internalclientset.Clientset, namespace string, selector map[string]string) ([]api.Pod, error) {
	list, err := client.Pods(namespace).List(api.ListOptions{
		FieldSelector: fields.Everything(),
		LabelSelector: labels.Set(selector).AsSelector(),
	})
	return list.Items, err
}

// AsVersionedObject converts a runtime.object to a versioned object.
func (c *Client) AsVersionedObject(obj runtime.Object) (runtime.Object, error) {
	json, err := runtime.Encode(runtime.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, err
	}
	versions := &runtime.VersionedObjects{}
	err = runtime.DecodeInto(c.Decoder(true), json, versions)
	return versions.First(), err
}

// waitForResources polls to get the current status of all pods, PVCs, and Services
// until all are ready or a timeout is reached
func (c *Client) waitForResources(timeout time.Duration, created Result) error {
	logger.WithFields(log.Fields{
		"_module":     "client",
		"_context":    "waitForResources",
		"timeout":     timeout,
		"num-objects": len(created),
	}).Info("Beginning wait for resources")
	client, _ := c.ClientSet()
	return wait.Poll(2*time.Second, timeout, func() (bool, error) {
		pods := []api.Pod{}
		services := []api.Service{}
		pvc := []api.PersistentVolumeClaim{}
		logger.WithFields(log.Fields{
			"_module":  "client",
			"_context": "waitForResources",
		}).Debug("Getting objects to watch from resource list")
		for _, v := range created {
			obj, err := c.AsVersionedObject(v.Object)
			if err != nil && !runtime.IsNotRegisteredError(err) {
				return false, err
			}
			switch value := obj.(type) {
			case (*v1.ReplicationController):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
					"selector":  value.Spec.Selector,
				}).Debug("Found ReplicationController. Getting Pods")
				list, err := getPods(client, value.Namespace, value.Spec.Selector)
				if err != nil {
					return false, err
				}
				pods = append(pods, list...)
			case (*v1.Pod):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
				}).Debug("Found pod. Updating info from API")
				pod, err := client.Pods(value.Namespace).Get(value.Name)
				if err != nil {
					return false, err
				}
				pods = append(pods, *pod)
			case (*extensions.Deployment):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
					"selector":  value.Spec.Selector.MatchLabels,
				}).Debug("Found deployment. Getting ReplicaSets")
				// Get the RS children first
				rs, err := client.ReplicaSets(value.Namespace).List(api.ListOptions{
					FieldSelector: fields.Everything(),
					LabelSelector: labels.Set(value.Spec.Selector.MatchLabels).AsSelector(),
				})
				if err != nil {
					return false, err
				}
				logger.WithFields(log.Fields{
					"_module":     "client",
					"_context":    "waitForResources",
					"namespace":   value.Namespace,
					"name":        value.Name,
					"num-objects": len(rs.Items),
				}).Debug("Found ReplicaSets from Deployment. Getting Pods")
				for _, r := range rs.Items {
					list, err := getPods(client, value.Namespace, r.Spec.Selector.MatchLabels)
					if err != nil {
						return false, err
					}
					pods = append(pods, list...)
				}
			case (*extensions.DaemonSet):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
					"selector":  value.Spec.Selector.MatchLabels,
				}).Debug("Found DaemonSet. Getting Pods")
				list, err := getPods(client, value.Namespace, value.Spec.Selector.MatchLabels)
				if err != nil {
					return false, err
				}
				pods = append(pods, list...)
			case (*apps.StatefulSet):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
					"selector":  value.Spec.Selector.MatchLabels,
				}).Debug("Found StatefulSet. Getting Pods")
				list, err := getPods(client, value.Namespace, value.Spec.Selector.MatchLabels)
				if err != nil {
					return false, err
				}
				pods = append(pods, list...)
			case (*extensions.ReplicaSet):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
					"selector":  value.Spec.Selector.MatchLabels,
				}).Debug("Found ReplicaSet. Getting Pods")
				list, err := getPods(client, value.Namespace, value.Spec.Selector.MatchLabels)
				if err != nil {
					return false, err
				}
				pods = append(pods, list...)
			case (*v1.PersistentVolumeClaim):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
				}).Debug("Found PersistentVolumeClaim. Updating info from API")
				claim, err := client.PersistentVolumeClaims(value.Namespace).Get(value.Name)
				if err != nil {
					return false, err
				}
				pvc = append(pvc, *claim)
			case (*v1.Service):
				logger.WithFields(log.Fields{
					"_module":   "client",
					"_context":  "waitForResources",
					"namespace": value.Namespace,
					"name":      value.Name,
				}).Debug("Found Service. Updating info from API")
				svc, err := client.Services(value.Namespace).Get(value.Name)
				if err != nil {
					return false, err
				}
				services = append(services, *svc)
			}
		}
		logger.WithFields(log.Fields{
			"_module":      "client",
			"_context":     "waitForResources",
			"num-pods":     len(pods),
			"num-services": len(services),
			"num-volumes":  len(pvc),
		}).Debug("Checking resources for ready state")
		return podsReady(pods) && servicesReady(services) && volumesReady(pvc), nil
	})
}

// waitForJob is a helper that waits for a job to complete.
//
// This operates on an event returned from a watcher.
func waitForJob(e watch.Event, name string) (bool, error) {
	o, ok := e.Object.(*batchinternal.Job)
	if !ok {
		return true, fmt.Errorf("Expected %s to be a *batch.Job, got %T", name, e.Object)
	}

	for _, c := range o.Status.Conditions {
		if c.Type == batchinternal.JobComplete && c.Status == api.ConditionTrue {
			return true, nil
		} else if c.Type == batchinternal.JobFailed && c.Status == api.ConditionTrue {
			return true, fmt.Errorf("Job failed: %s", c.Reason)
		}
	}
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "waitForJob",
		"name":      name,
		"active":    o.Status.Active,
		"failed":    o.Status.Failed,
		"succeeded": o.Status.Succeeded,
	}).Info("Waiting on job completion")
	return false, nil
}

// scrubValidationError removes kubectl info from the message
func scrubValidationError(err error) error {
	if err == nil {
		return nil
	}
	const stopValidateMessage = "if you choose to ignore these errors, turn validation off with --validate=false"

	if strings.Contains(err.Error(), stopValidateMessage) {
		return goerrors.New(strings.Replace(err.Error(), "; "+stopValidateMessage, "", -1))
	}
	return err
}

// WaitAndGetCompletedPodPhase waits up to a timeout until a pod enters a completed phase
// and returns said phase (PodSucceeded or PodFailed qualify)
func (c *Client) WaitAndGetCompletedPodPhase(namespace string, reader io.Reader, timeout time.Duration) (api.PodPhase, error) {
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "WaitAndGetCompletedPodPhase",
		"namespace": namespace,
	}).Debug("Creating pod from manifest")
	infos, err := c.Build(namespace, reader)
	if err != nil {
		return api.PodUnknown, err
	}
	info := infos[0]

	kind := info.Mapping.GroupVersionKind.Kind
	if kind != "Pod" {
		return api.PodUnknown, fmt.Errorf("%s is not a Pod", info.Name)
	}

	if err := watchPodUntilComplete(timeout, info); err != nil {
		return api.PodUnknown, err
	}

	if err := info.Get(); err != nil {
		return api.PodUnknown, err
	}
	status := info.Object.(*api.Pod).Status.Phase

	return status, nil
}

func watchPodUntilComplete(timeout time.Duration, info *resource.Info) error {
	w, err := resource.NewHelper(info.Client, info.Mapping).WatchSingle(info.Namespace, info.Name, info.ResourceVersion)
	if err != nil {
		return err
	}
	logger.WithFields(log.Fields{
		"_module":   "client",
		"_context":  "watchPodUntilComplete",
		"namespace": info.Namespace,
		"name":      info.Name,
		"timeout":   timeout,
	}).Info("Watching pod for completion")
	_, err = watch.Until(timeout, w, func(e watch.Event) (bool, error) {
		return conditions.PodCompleted(e)
	})

	return err
}
