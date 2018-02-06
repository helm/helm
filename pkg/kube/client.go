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
	"log"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batch "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
	batchinternal "k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/core"
	conditions "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/kubectl/validation"
	"k8s.io/kubernetes/pkg/printers"
)

const (
	// MissingGetHeader is added to Get's outout when a resource is not found.
	MissingGetHeader = "==> MISSING\nKIND\t\tNAME\n"
)

// ErrNoObjectsVisited indicates that during a visit operation, no matching objects were found.
var ErrNoObjectsVisited = goerrors.New("no objects visited")

// Client represents a client capable of communicating with the Kubernetes API.
type Client struct {
	cmdutil.Factory
	// SchemaCacheDir is the path for loading cached schema.
	SchemaCacheDir string

	Log func(string, ...interface{})
}

// New creates a new Client.
func New(config clientcmd.ClientConfig) *Client {
	return &Client{
		Factory:        cmdutil.NewFactory(config),
		SchemaCacheDir: clientcmd.RecommendedSchemaFile,
		Log:            func(_ string, _ ...interface{}) {},
	}
}

// ResourceActorFunc performs an action on a single resource.
type ResourceActorFunc func(*resource.Info) error

// Create creates Kubernetes resources from an io.reader.
//
// Namespace will set the namespace.
func (c *Client) Create(namespace string, reader io.Reader, timeout int64, shouldWait bool) error {
	client, err := c.ClientSet()
	if err != nil {
		return err
	}
	if err := ensureNamespace(client, namespace); err != nil {
		return err
	}
	c.Log("building resources from manifest")
	infos, buildErr := c.BuildUnstructured(namespace, reader)
	if buildErr != nil {
		return buildErr
	}
	c.Log("creating %d resource(s)", len(infos))
	if err := perform(infos, createResource); err != nil {
		return err
	}
	if shouldWait {
		return c.waitForResources(time.Duration(timeout)*time.Second, infos)
	}
	return nil
}

func (c *Client) newBuilder(namespace string, reader io.Reader) *resource.Result {
	return c.NewBuilder().
		Internal().
		ContinueOnError().
		Schema(c.validator()).
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(reader, "").
		Flatten().
		Do()
}

func (c *Client) validator() validation.Schema {
	schema, err := c.Validator(true)
	if err != nil {
		c.Log("warning: failed to load schema: %s", err)
	}
	return schema
}

// BuildUnstructured validates for Kubernetes objects and returns unstructured infos.
func (c *Client) BuildUnstructured(namespace string, reader io.Reader) (Result, error) {
	var result Result

	result, err := c.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(reader, "").
		Flatten().
		Do().Infos()
	return result, scrubValidationError(err)
}

// Build validates for Kubernetes objects and returns resource Infos from a io.Reader.
func (c *Client) Build(namespace string, reader io.Reader) (Result, error) {
	var result Result
	result, err := c.newBuilder(namespace, reader).Infos()
	return result, scrubValidationError(err)
}

// Get gets Kubernetes resources as pretty-printed string.
//
// Namespace will set the namespace.
func (c *Client) Get(namespace string, reader io.Reader) (string, error) {
	// Since we don't know what order the objects come in, let's group them by the types, so
	// that when we print them, they come out looking good (headers apply to subgroups, etc.).
	objs := make(map[string][]runtime.Object)
	infos, err := c.BuildUnstructured(namespace, reader)
	if err != nil {
		return "", err
	}

	var objPods = make(map[string][]core.Pod)

	missing := []string{}
	err = perform(infos, func(info *resource.Info) error {
		c.Log("Doing get for %s: %q", info.Mapping.GroupVersionKind.Kind, info.Name)
		if err := info.Get(); err != nil {
			c.Log("WARNING: Failed Get for resource %q: %s", info.Name, err)
			missing = append(missing, fmt.Sprintf("%v\t\t%s", info.Mapping.Resource, info.Name))
			return nil
		}

		// Use APIVersion/Kind as grouping mechanism. I'm not sure if you can have multiple
		// versions per cluster, but this certainly won't hurt anything, so let's be safe.
		gvk := info.ResourceMapping().GroupVersionKind
		vk := gvk.Version + "/" + gvk.Kind
		objs[vk] = append(objs[vk], info.Object)

		//Get the relation pods
		objPods, err = c.getSelectRelationPod(info, objPods)
		if err != nil {
			c.Log("Warning: get the relation pod is failed, err:%s", err.Error())
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	//here, we will add the objPods to the objs
	for key, podItems := range objPods {
		for i := range podItems {
			objs[key+"(related)"] = append(objs[key+"(related)"], &podItems[i])
		}
	}

	// Ok, now we have all the objects grouped by types (say, by v1/Pod, v1/Service, etc.), so
	// spin through them and print them. Printer is cool since it prints the header only when
	// an object type changes, so we can just rely on that. Problem is it doesn't seem to keep
	// track of tab widths.
	buf := new(bytes.Buffer)
	p, _ := c.Printer(nil, printers.PrintOptions{})
	for t, ot := range objs {
		if _, err = buf.WriteString("==> " + t + "\n"); err != nil {
			return "", err
		}
		for _, o := range ot {
			if err := p.PrintObj(o, buf); err != nil {
				c.Log("failed to print object type %s, object: %q :\n %v", t, o, err)
				return "", err
			}
		}
		if _, err := buf.WriteString("\n"); err != nil {
			return "", err
		}
	}
	if len(missing) > 0 {
		buf.WriteString(MissingGetHeader)
		for _, s := range missing {
			fmt.Fprintln(buf, s)
		}
	}
	return buf.String(), nil
}

// Update reads in the current configuration and a target configuration from io.reader
// and creates resources that don't already exists, updates resources that have been modified
// in the target configuration and deletes resources from the current configuration that are
// not present in the target configuration.
//
// Namespace will set the namespaces.
func (c *Client) Update(namespace string, originalReader, targetReader io.Reader, force bool, recreate bool, timeout int64, shouldWait bool) error {
	original, err := c.BuildUnstructured(namespace, originalReader)
	if err != nil {
		return fmt.Errorf("failed decoding reader into objects: %s", err)
	}

	c.Log("building resources from updated manifest")
	target, err := c.BuildUnstructured(namespace, targetReader)
	if err != nil {
		return fmt.Errorf("failed decoding reader into objects: %s", err)
	}

	updateErrors := []string{}

	c.Log("checking %d resources for changes", len(target))
	err = target.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(info.Client, info.Mapping)
		if _, err := helper.Get(info.Namespace, info.Name, info.Export); err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("Could not get information about the resource: %s", err)
			}

			// Since the resource does not exist, create it.
			if err := createResource(info); err != nil {
				return fmt.Errorf("failed to create resource: %s", err)
			}

			kind := info.Mapping.GroupVersionKind.Kind
			c.Log("Created a new %s called %q\n", kind, info.Name)
			return nil
		}

		originalInfo := original.Get(info)
		if originalInfo == nil {
			kind := info.Mapping.GroupVersionKind.Kind
			return fmt.Errorf("no %s with the name %q found", kind, info.Name)
		}

		if err := updateResource(c, info, originalInfo.Object, force, recreate); err != nil {
			c.Log("error updating the resource %q:\n\t %v", info.Name, err)
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
		c.Log("Deleting %q in %s...", info.Name, info.Namespace)
		if err := deleteResource(c, info); err != nil {
			c.Log("Failed to delete %q, err: %s", info.Name, err)
		}
	}
	if shouldWait {
		return c.waitForResources(time.Duration(timeout)*time.Second, target)
	}
	return nil
}

// Delete deletes Kubernetes resources from an io.reader.
//
// Namespace will set the namespace.
func (c *Client) Delete(namespace string, reader io.Reader) error {
	infos, err := c.BuildUnstructured(namespace, reader)
	if err != nil {
		return err
	}
	return perform(infos, func(info *resource.Info) error {
		c.Log("Starting delete for %q %s", info.Name, info.Mapping.GroupVersionKind.Kind)
		err := deleteResource(c, info)
		return c.skipIfNotFound(err)
	})
}

func (c *Client) skipIfNotFound(err error) error {
	if errors.IsNotFound(err) {
		c.Log("%v", err)
		return nil
	}
	return err
}

func (c *Client) watchTimeout(t time.Duration) ResourceActorFunc {
	return func(info *resource.Info) error {
		return c.watchUntilReady(t, info)
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
	infos, err := c.Build(namespace, reader)
	if err != nil {
		return err
	}
	// For jobs, there's also the option to do poll c.Jobs(namespace).Get():
	// https://github.com/adamreese/kubernetes/blob/master/test/e2e/job.go#L291-L300
	return perform(infos, c.watchTimeout(time.Duration(timeout)*time.Second))
}

func perform(infos Result, fn ResourceActorFunc) error {
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
			return resource.NewHelper(info.Client, info.Mapping).Delete(info.Namespace, info.Name)
		}
		return err
	}
	c.Log("Using reaper for deleting %q", info.Name)
	return reaper.Stop(info.Namespace, info.Name, 0, nil)
}

func createPatch(mapping *meta.RESTMapping, target, current runtime.Object) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(current)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing current configuration: %s", err)
	}
	newData, err := json.Marshal(target)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing target configuration: %s", err)
	}

	// While different objects need different merge types, the parent function
	// that calls this does not try to create a patch when the data (first
	// returned object) is nil. We can skip calculating the the merge type as
	// the returned merge type is ignored.
	if apiequality.Semantic.DeepEqual(oldData, newData) {
		return nil, types.StrategicMergePatchType, nil
	}

	// Get a versioned object
	versionedObject, err := mapping.ConvertToVersion(target, mapping.GroupVersionKind.GroupVersion())

	// Unstructured objects, such as CRDs, may not have an not registered error
	// returned from ConvertToVersion. Anything that's unstructured should
	// use the jsonpatch.CreateMergePatch. Strategic Merge Patch is not supported
	// on objects like CRDs.
	_, isUnstructured := versionedObject.(runtime.Unstructured)

	switch {
	case runtime.IsNotRegisteredError(err), isUnstructured:
		// fall back to generic JSON merge patch
		patch, err := jsonpatch.CreateMergePatch(oldData, newData)
		return patch, types.MergePatchType, err
	case err != nil:
		return nil, types.StrategicMergePatchType, fmt.Errorf("failed to get versionedObject: %s", err)
	default:
		patch, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, versionedObject)
		return patch, types.StrategicMergePatchType, err
	}
}

func updateResource(c *Client, target *resource.Info, currentObj runtime.Object, force bool, recreate bool) error {
	patch, patchType, err := createPatch(target.Mapping, target.Object, currentObj)
	if err != nil {
		return fmt.Errorf("failed to create patch: %s", err)
	}
	if patch == nil {
		c.Log("Looks like there are no changes for %s %q", target.Mapping.GroupVersionKind.Kind, target.Name)
		// This needs to happen to make sure that tiller has the latest info from the API
		// Otherwise there will be no labels and other functions that use labels will panic
		if err := target.Get(); err != nil {
			return fmt.Errorf("error trying to refresh resource information: %v", err)
		}
	} else {
		// send patch to server
		helper := resource.NewHelper(target.Client, target.Mapping)

		obj, err := helper.Patch(target.Namespace, target.Name, patchType, patch)
		if err != nil {
			kind := target.Mapping.GroupVersionKind.Kind
			log.Printf("Cannot patch %s: %q (%v)", kind, target.Name, err)

			if force {
				// Attempt to delete...
				if err := deleteResource(c, target); err != nil {
					return err
				}
				log.Printf("Deleted %s: %q", kind, target.Name)

				// ... and recreate
				if err := createResource(target); err != nil {
					return fmt.Errorf("Failed to recreate resource: %s", err)
				}
				log.Printf("Created a new %s called %q\n", kind, target.Name)

				// No need to refresh the target, as we recreated the resource based
				// on it. In addition, it might not exist yet and a call to `Refresh`
				// may fail.
			} else {
				log.Print("Use --force to force recreation of the resource")
				return err
			}
		} else {
			// When patch succeeds without needing to recreate, refresh target.
			target.Refresh(obj, true)
		}
	}

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

	client, err := c.ClientSet()
	if err != nil {
		return err
	}

	pods, err := client.Core().Pods(target.Namespace).List(metav1.ListOptions{
		FieldSelector: fields.Everything().String(),
		LabelSelector: labels.Set(selector).AsSelector().String(),
	})
	if err != nil {
		return err
	}

	// Restart pods
	for _, pod := range pods.Items {
		c.Log("Restarting pod: %v/%v", pod.Namespace, pod.Name)

		// Delete each pod for get them restarted with changed spec.
		if err := client.Core().Pods(pod.Namespace).Delete(pod.Name, metav1.NewPreconditionDeleteOptions(string(pod.UID))); err != nil {
			return err
		}
	}
	return nil
}

func getSelectorFromObject(obj runtime.Object) (map[string]string, error) {
	switch typed := obj.(type) {

	case *v1.ReplicationController:
		return typed.Spec.Selector, nil

	case *extv1beta1.ReplicaSet:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1.ReplicaSet:
		return typed.Spec.Selector.MatchLabels, nil

	case *extv1beta1.Deployment:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1beta1.Deployment:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1beta2.Deployment:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1.Deployment:
		return typed.Spec.Selector.MatchLabels, nil

	case *extv1beta1.DaemonSet:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1beta2.DaemonSet:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1.DaemonSet:
		return typed.Spec.Selector.MatchLabels, nil

	case *batch.Job:
		return typed.Spec.Selector.MatchLabels, nil

	case *appsv1beta1.StatefulSet:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1beta2.StatefulSet:
		return typed.Spec.Selector.MatchLabels, nil
	case *appsv1.StatefulSet:
		return typed.Spec.Selector.MatchLabels, nil

	default:
		return nil, fmt.Errorf("Unsupported kind when getting selector: %v", obj)
	}
}

func (c *Client) watchUntilReady(timeout time.Duration, info *resource.Info) error {
	w, err := resource.NewHelper(info.Client, info.Mapping).WatchSingle(info.Namespace, info.Name, info.ResourceVersion)
	if err != nil {
		return err
	}

	kind := info.Mapping.GroupVersionKind.Kind
	c.Log("Watching for changes to %s %s with timeout of %v", kind, info.Name, timeout)

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
			c.Log("Add/Modify event for %s: %v", info.Name, e.Type)
			if kind == "Job" {
				return c.waitForJob(e, info.Name)
			}
			return true, nil
		case watch.Deleted:
			c.Log("Deleted event for %s", info.Name)
			return true, nil
		case watch.Error:
			// Handle error and return with an error.
			c.Log("Error event for %s", info.Name)
			return true, fmt.Errorf("Failed to deploy %s", info.Name)
		default:
			return false, nil
		}
	})
	return err
}

// AsVersionedObject converts a runtime.object to a versioned object.
func (c *Client) AsVersionedObject(obj runtime.Object) (runtime.Object, error) {
	json, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, err
	}
	versions := &runtime.VersionedObjects{}
	err = runtime.DecodeInto(c.Decoder(true), json, versions)
	return versions.First(), err
}

// waitForJob is a helper that waits for a job to complete.
//
// This operates on an event returned from a watcher.
func (c *Client) waitForJob(e watch.Event, name string) (bool, error) {
	o, ok := e.Object.(*batchinternal.Job)
	if !ok {
		return true, fmt.Errorf("Expected %s to be a *batch.Job, got %T", name, e.Object)
	}

	for _, c := range o.Status.Conditions {
		if c.Type == batchinternal.JobComplete && c.Status == core.ConditionTrue {
			return true, nil
		} else if c.Type == batchinternal.JobFailed && c.Status == core.ConditionTrue {
			return true, fmt.Errorf("Job failed: %s", c.Reason)
		}
	}

	c.Log("%s: Jobs active: %d, jobs failed: %d, jobs succeeded: %d", name, o.Status.Active, o.Status.Failed, o.Status.Succeeded)
	return false, nil
}

// scrubValidationError removes kubectl info from the message.
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
// and returns said phase (PodSucceeded or PodFailed qualify).
func (c *Client) WaitAndGetCompletedPodPhase(namespace string, reader io.Reader, timeout time.Duration) (core.PodPhase, error) {
	infos, err := c.Build(namespace, reader)
	if err != nil {
		return core.PodUnknown, err
	}
	info := infos[0]

	kind := info.Mapping.GroupVersionKind.Kind
	if kind != "Pod" {
		return core.PodUnknown, fmt.Errorf("%s is not a Pod", info.Name)
	}

	if err := c.watchPodUntilComplete(timeout, info); err != nil {
		return core.PodUnknown, err
	}

	if err := info.Get(); err != nil {
		return core.PodUnknown, err
	}
	status := info.Object.(*core.Pod).Status.Phase

	return status, nil
}

func (c *Client) watchPodUntilComplete(timeout time.Duration, info *resource.Info) error {
	w, err := resource.NewHelper(info.Client, info.Mapping).WatchSingle(info.Namespace, info.Name, info.ResourceVersion)
	if err != nil {
		return err
	}

	c.Log("Watching pod %s for completion with timeout of %v", info.Name, timeout)
	_, err = watch.Until(timeout, w, func(e watch.Event) (bool, error) {
		return conditions.PodCompleted(e)
	})

	return err
}

//get an kubernetes resources's relation pods
// kubernetes resource used select labels to relate pods
func (c *Client) getSelectRelationPod(info *resource.Info, objPods map[string][]core.Pod) (map[string][]core.Pod, error) {
	if info == nil {
		return objPods, nil
	}

	c.Log("get relation pod of object: %s/%s/%s", info.Namespace, info.Mapping.GroupVersionKind.Kind, info.Name)

	versioned, err := c.AsVersionedObject(info.Object)
	if runtime.IsNotRegisteredError(err) {
		return objPods, nil
	}
	if err != nil {
		return objPods, err
	}

	// We can ignore this error because it will only error if it isn't a type that doesn't
	// have pods. In that case, we don't care
	selector, _ := getSelectorFromObject(versioned)

	selectorString := labels.Set(selector).AsSelector().String()

	// If we have an empty selector, this likely is a service or config map, so bail out now
	if selectorString == "" {
		return objPods, nil
	}

	client, _ := c.ClientSet()

	pods, err := client.Core().Pods(info.Namespace).List(metav1.ListOptions{
		FieldSelector: fields.Everything().String(),
		LabelSelector: labels.Set(selector).AsSelector().String(),
	})
	if err != nil {
		return objPods, err
	}

	for _, pod := range pods.Items {
		if pod.APIVersion == "" {
			pod.APIVersion = "v1"
		}

		if pod.Kind == "" {
			pod.Kind = "Pod"
		}
		vk := pod.GroupVersionKind().Version + "/" + pod.GroupVersionKind().Kind

		if !isFoundPod(objPods[vk], pod) {
			objPods[vk] = append(objPods[vk], pod)
		}
	}
	return objPods, nil
}

func isFoundPod(podItem []core.Pod, pod core.Pod) bool {
	for _, value := range podItem {
		if (value.Namespace == pod.Namespace) && (value.Name == pod.Name) {
			return true
		}
	}
	return false
}
