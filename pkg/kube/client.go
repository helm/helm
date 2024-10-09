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

package kube // import "helm.sh/helm/v3/pkg/kube"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	multierror "github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	cachetools "k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// ErrNoObjectsVisited indicates that during a visit operation, no matching objects were found.
var ErrNoObjectsVisited = errors.New("no objects visited")

var metadataAccessor = meta.NewAccessor()

// ManagedFieldsManager is the name of the manager of Kubernetes managedFields
// first introduced in Kubernetes 1.18
var ManagedFieldsManager string

// Client represents a client capable of communicating with the Kubernetes API.
type Client struct {
	// Factory provides a minimal version of the kubectl Factory interface. If
	// you need the full Factory you can type switch to the full interface.
	// Since Kubernetes Go API does not provide backwards compatibility across
	// minor versions, this API does not follow Helm backwards compatibility.
	// Helm is exposing Kubernetes in this property and cannot guarantee this
	// will not change. The minimal interface only has the functions that Helm
	// needs. The smaller surface area of the interface means there is a lower
	// chance of it changing.
	Factory Factory
	Log     func(string, ...interface{})
	// Namespace allows to bypass the kubeconfig file for the choice of the namespace
	Namespace string

	kubeClient *kubernetes.Clientset
}

func init() {
	// Add CRDs to the scheme. They are missing by default.
	if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
		// This should never happen.
		panic(err)
	}
	if err := apiextv1beta1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

// New creates a new Client.
func New(getter genericclioptions.RESTClientGetter) *Client {
	if getter == nil {
		getter = genericclioptions.NewConfigFlags(true)
	}
	return &Client{
		Factory: cmdutil.NewFactory(getter),
		Log:     nopLogger,
	}
}

var nopLogger = func(_ string, _ ...interface{}) {}

// getKubeClient get or create a new KubernetesClientSet
func (c *Client) getKubeClient() (*kubernetes.Clientset, error) {
	var err error
	if c.kubeClient == nil {
		c.kubeClient, err = c.Factory.KubernetesClientSet()
	}

	return c.kubeClient, err
}

// IsReachable tests connectivity to the cluster.
func (c *Client) IsReachable() error {
	client, err := c.getKubeClient()
	if err == genericclioptions.ErrEmptyConfig {
		// re-replace kubernetes ErrEmptyConfig error with a friendly error
		// moar workarounds for Kubernetes API breaking.
		return errors.New("Kubernetes cluster unreachable")
	}
	if err != nil {
		return errors.Wrap(err, "Kubernetes cluster unreachable")
	}
	if _, err := client.ServerVersion(); err != nil {
		return errors.Wrap(err, "Kubernetes cluster unreachable")
	}
	return nil
}

// Create creates Kubernetes resources specified in the resource list.
func (c *Client) Create(resources ResourceList) (*Result, error) {
	c.Log("creating %d resource(s)", len(resources))
	if err := perform(resources, createResource); err != nil {
		return nil, err
	}
	return &Result{Created: resources}, nil
}

func transformRequests(req *rest.Request) {
	tableParam := strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
		"application/json",
	}, ",")
	req.SetHeader("Accept", tableParam)

	// if sorting, ensure we receive the full object in order to introspect its fields via jsonpath
	req.Param("includeObject", "Object")
}

// Get retrieves the resource objects supplied. If related is set to true the
// related pods are fetched as well. If the passed in resources are a table kind
// the related resources will also be fetched as kind=table.
func (c *Client) Get(resources ResourceList, related bool) (map[string][]runtime.Object, error) {
	buf := new(bytes.Buffer)
	objs := make(map[string][]runtime.Object)

	podSelectors := []map[string]string{}
	err := resources.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		gvk := info.ResourceMapping().GroupVersionKind
		vk := gvk.Version + "/" + gvk.Kind
		obj, err := getResource(info)
		if err != nil {
			fmt.Fprintf(buf, "Get resource %s failed, err:%v\n", info.Name, err)
		} else {
			objs[vk] = append(objs[vk], obj)

			// Only fetch related pods if they are requested
			if related {
				// Discover if the existing object is a table. If it is, request
				// the pods as Tables. Otherwise request them normally.
				objGVK := obj.GetObjectKind().GroupVersionKind()
				var isTable bool
				if objGVK.Kind == "Table" {
					isTable = true
				}

				objs, err = c.getSelectRelationPod(info, objs, isTable, &podSelectors)
				if err != nil {
					c.Log("Warning: get the relation pod is failed, err:%s", err.Error())
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return objs, nil
}

func (c *Client) getSelectRelationPod(info *resource.Info, objs map[string][]runtime.Object, table bool, podSelectors *[]map[string]string) (map[string][]runtime.Object, error) {
	if info == nil {
		return objs, nil
	}
	c.Log("get relation pod of object: %s/%s/%s", info.Namespace, info.Mapping.GroupVersionKind.Kind, info.Name)
	selector, ok, _ := getSelectorFromObject(info.Object)
	if !ok {
		return objs, nil
	}

	for index := range *podSelectors {
		if reflect.DeepEqual((*podSelectors)[index], selector) {
			// check if pods for selectors are already added. This avoids duplicate printing of pods
			return objs, nil
		}
	}

	*podSelectors = append(*podSelectors, selector)

	var infos []*resource.Info
	var err error
	if table {
		infos, err = c.Factory.NewBuilder().
			Unstructured().
			ContinueOnError().
			NamespaceParam(info.Namespace).
			DefaultNamespace().
			ResourceTypes("pods").
			LabelSelector(labels.Set(selector).AsSelector().String()).
			TransformRequests(transformRequests).
			Do().Infos()
		if err != nil {
			return objs, err
		}
	} else {
		infos, err = c.Factory.NewBuilder().
			Unstructured().
			ContinueOnError().
			NamespaceParam(info.Namespace).
			DefaultNamespace().
			ResourceTypes("pods").
			LabelSelector(labels.Set(selector).AsSelector().String()).
			Do().Infos()
		if err != nil {
			return objs, err
		}
	}
	vk := "v1/Pod(related)"

	for _, info := range infos {
		objs[vk] = append(objs[vk], info.Object)
	}
	return objs, nil
}

func getSelectorFromObject(obj runtime.Object) (map[string]string, bool, error) {
	typed := obj.(*unstructured.Unstructured)
	kind := typed.Object["kind"]
	switch kind {
	case "ReplicaSet", "Deployment", "StatefulSet", "DaemonSet", "Job":
		return unstructured.NestedStringMap(typed.Object, "spec", "selector", "matchLabels")
	case "ReplicationController":
		return unstructured.NestedStringMap(typed.Object, "spec", "selector")
	default:
		return nil, false, nil
	}
}

func getResource(info *resource.Info) (runtime.Object, error) {
	obj, err := resource.NewHelper(info.Client, info.Mapping).Get(info.Namespace, info.Name)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// Wait waits up to the given timeout for the specified resources to be ready.
func (c *Client) Wait(resources ResourceList, timeout time.Duration) error {
	cs, err := c.getKubeClient()
	if err != nil {
		return err
	}
	checker := NewReadyChecker(cs, c.Log, PausedAsReady(true))
	w := waiter{
		c:       checker,
		log:     c.Log,
		timeout: timeout,
	}
	return w.waitForResources(resources)
}

// WaitWithJobs wait up to the given timeout for the specified resources to be ready, including jobs.
func (c *Client) WaitWithJobs(resources ResourceList, timeout time.Duration) error {
	cs, err := c.getKubeClient()
	if err != nil {
		return err
	}
	checker := NewReadyChecker(cs, c.Log, PausedAsReady(true), CheckJobs(true))
	w := waiter{
		c:       checker,
		log:     c.Log,
		timeout: timeout,
	}
	return w.waitForResources(resources)
}

// WaitForDelete wait up to the given timeout for the specified resources to be deleted.
func (c *Client) WaitForDelete(resources ResourceList, timeout time.Duration) error {
	w := waiter{
		log:     c.Log,
		timeout: timeout,
	}
	return w.waitForDeletedResources(resources)
}

func (c *Client) namespace() string {
	if c.Namespace != "" {
		return c.Namespace
	}
	if ns, _, err := c.Factory.ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return v1.NamespaceDefault
}

// newBuilder returns a new resource builder for structured api objects.
func (c *Client) newBuilder() *resource.Builder {
	return c.Factory.NewBuilder().
		ContinueOnError().
		NamespaceParam(c.namespace()).
		DefaultNamespace().
		Flatten()
}

// Build validates for Kubernetes objects and returns unstructured infos.
func (c *Client) Build(reader io.Reader, validate bool) (ResourceList, error) {
	validationDirective := metav1.FieldValidationIgnore
	if validate {
		validationDirective = metav1.FieldValidationStrict
	}

	schema, err := c.Factory.Validator(validationDirective)
	if err != nil {
		return nil, err
	}
	result, err := c.newBuilder().
		Unstructured().
		Schema(schema).
		Stream(reader, "").
		Do().Infos()
	return result, scrubValidationError(err)
}

// BuildTable validates for Kubernetes objects and returns unstructured infos.
// The returned kind is a Table.
func (c *Client) BuildTable(reader io.Reader, validate bool) (ResourceList, error) {
	validationDirective := metav1.FieldValidationIgnore
	if validate {
		validationDirective = metav1.FieldValidationStrict
	}

	schema, err := c.Factory.Validator(validationDirective)
	if err != nil {
		return nil, err
	}
	result, err := c.newBuilder().
		Unstructured().
		Schema(schema).
		Stream(reader, "").
		TransformRequests(transformRequests).
		Do().Infos()
	return result, scrubValidationError(err)
}

// Update takes the current list of objects and target list of objects and
// creates resources that don't already exist, updates resources that have been
// modified in the target configuration, and deletes resources from the current
// configuration that are not present in the target configuration. If an error
// occurs, a Result will still be returned with the error, containing all
// resource updates, creations, and deletions that were attempted. These can be
// used for cleanup or other logging purposes.
func (c *Client) Update(original, target ResourceList, force bool) (*Result, error) {
	updateErrors := []string{}
	res := &Result{}

	c.Log("checking %d resources for changes", len(target))
	err := target.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(info.Client, info.Mapping).WithFieldManager(getManagedFieldsManager())
		if _, err := helper.Get(info.Namespace, info.Name); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrap(err, "could not get information about the resource")
			}

			// Append the created resource to the results, even if something fails
			res.Created = append(res.Created, info)

			// Since the resource does not exist, create it.
			if err := createResource(info); err != nil {
				return errors.Wrap(err, "failed to create resource")
			}

			kind := info.Mapping.GroupVersionKind.Kind
			c.Log("Created a new %s called %q in %s\n", kind, info.Name, info.Namespace)
			return nil
		}

		originalInfo := original.Get(info)
		if originalInfo == nil {
			kind := info.Mapping.GroupVersionKind.Kind
			return errors.Errorf("no %s with the name %q found", kind, info.Name)
		}

		if err := updateResource(c, info, originalInfo.Object, force); err != nil {
			c.Log("error updating the resource %q:\n\t %v", info.Name, err)
			updateErrors = append(updateErrors, err.Error())
		}
		// Because we check for errors later, append the info regardless
		res.Updated = append(res.Updated, info)

		return nil
	})

	switch {
	case err != nil:
		return res, err
	case len(updateErrors) != 0:
		return res, errors.Errorf(strings.Join(updateErrors, " && "))
	}

	for _, info := range original.Difference(target) {
		c.Log("Deleting %s %q in namespace %s...", info.Mapping.GroupVersionKind.Kind, info.Name, info.Namespace)

		if err := info.Get(); err != nil {
			c.Log("Unable to get obj %q, err: %s", info.Name, err)
			continue
		}
		annotations, err := metadataAccessor.Annotations(info.Object)
		if err != nil {
			c.Log("Unable to get annotations on %q, err: %s", info.Name, err)
		}
		if annotations != nil && annotations[ResourcePolicyAnno] == KeepPolicy {
			c.Log("Skipping delete of %q due to annotation [%s=%s]", info.Name, ResourcePolicyAnno, KeepPolicy)
			continue
		}
		if err := deleteResource(info, metav1.DeletePropagationBackground); err != nil {
			c.Log("Failed to delete %q, err: %s", info.ObjectName(), err)
			continue
		}
		res.Deleted = append(res.Deleted, info)
	}
	return res, nil
}

// Delete deletes Kubernetes resources specified in the resources list with
// background cascade deletion. It will attempt to delete all resources even
// if one or more fail and collect any errors. All successfully deleted items
// will be returned in the `Deleted` ResourceList that is part of the result.
func (c *Client) Delete(resources ResourceList) (*Result, []error) {
	return rdelete(c, resources, metav1.DeletePropagationBackground)
}

// Delete deletes Kubernetes resources specified in the resources list with
// given deletion propagation policy. It will attempt to delete all resources even
// if one or more fail and collect any errors. All successfully deleted items
// will be returned in the `Deleted` ResourceList that is part of the result.
func (c *Client) DeleteWithPropagationPolicy(resources ResourceList, policy metav1.DeletionPropagation) (*Result, []error) {
	return rdelete(c, resources, policy)
}

func rdelete(c *Client, resources ResourceList, propagation metav1.DeletionPropagation) (*Result, []error) {
	var errs []error
	res := &Result{}
	mtx := sync.Mutex{}
	err := perform(resources, func(info *resource.Info) error {
		c.Log("Starting delete for %q %s", info.Name, info.Mapping.GroupVersionKind.Kind)
		err := deleteResource(info, propagation)
		if err == nil || apierrors.IsNotFound(err) {
			if err != nil {
				c.Log("Ignoring delete failure for %q %s: %v", info.Name, info.Mapping.GroupVersionKind, err)
			}
			mtx.Lock()
			defer mtx.Unlock()
			res.Deleted = append(res.Deleted, info)
			return nil
		}
		mtx.Lock()
		defer mtx.Unlock()
		// Collect the error and continue on
		errs = append(errs, err)
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNoObjectsVisited) {
			err = fmt.Errorf("object not found, skipping delete: %w", err)
		}
		errs = append(errs, err)
	}
	if errs != nil {
		return nil, errs
	}
	return res, nil
}

func (c *Client) watchTimeout(t time.Duration) func(*resource.Info) error {
	return func(info *resource.Info) error {
		return c.watchUntilReady(t, info)
	}
}

// WatchUntilReady watches the resources given and waits until it is ready.
//
// This method is mainly for hook implementations. It watches for a resource to
// hit a particular milestone. The milestone depends on the Kind.
//
// For most kinds, it checks to see if the resource is marked as Added or Modified
// by the Kubernetes event stream. For some kinds, it does more:
//
//   - Jobs: A job is marked "Ready" when it has successfully completed. This is
//     ascertained by watching the Status fields in a job's output.
//   - Pods: A pod is marked "Ready" when it has successfully completed. This is
//     ascertained by watching the status.phase field in a pod's output.
//
// Handling for other kinds will be added as necessary.
func (c *Client) WatchUntilReady(resources ResourceList, timeout time.Duration) error {
	// For jobs, there's also the option to do poll c.Jobs(namespace).Get():
	// https://github.com/adamreese/kubernetes/blob/master/test/e2e/job.go#L291-L300
	return perform(resources, c.watchTimeout(timeout))
}

func perform(infos ResourceList, fn func(*resource.Info) error) error {
	var result error

	if len(infos) == 0 {
		return ErrNoObjectsVisited
	}

	errs := make(chan error)
	go batchPerform(infos, fn, errs)

	for range infos {
		err := <-errs
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}

// getManagedFieldsManager returns the manager string. If one was set it will be returned.
// Otherwise, one is calculated based on the name of the binary.
func getManagedFieldsManager() string {

	// When a manager is explicitly set use it
	if ManagedFieldsManager != "" {
		return ManagedFieldsManager
	}

	// When no manager is set and no calling application can be found it is unknown
	if len(os.Args[0]) == 0 {
		return "unknown"
	}

	// When there is an application that can be determined and no set manager
	// use the base name. This is one of the ways Kubernetes libs handle figuring
	// names out.
	return filepath.Base(os.Args[0])
}

func batchPerform(infos ResourceList, fn func(*resource.Info) error, errs chan<- error) {
	var kind string
	var wg sync.WaitGroup
	for _, info := range infos {
		currentKind := info.Object.GetObjectKind().GroupVersionKind().Kind
		if kind != currentKind {
			wg.Wait()
			kind = currentKind
		}
		wg.Add(1)
		go func(i *resource.Info) {
			errs <- fn(i)
			wg.Done()
		}(info)
	}
}

func createResource(info *resource.Info) error {
	obj, err := resource.NewHelper(info.Client, info.Mapping).WithFieldManager(getManagedFieldsManager()).Create(info.Namespace, true, info.Object)
	if err != nil {
		return err
	}
	return info.Refresh(obj, true)
}

func deleteResource(info *resource.Info, policy metav1.DeletionPropagation) error {
	opts := &metav1.DeleteOptions{PropagationPolicy: &policy}
	_, err := resource.NewHelper(info.Client, info.Mapping).WithFieldManager(getManagedFieldsManager()).DeleteWithOptions(info.Namespace, info.Name, opts)
	return err
}

func createPatch(target *resource.Info, current runtime.Object) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(current)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "serializing current configuration")
	}
	newData, err := json.Marshal(target.Object)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "serializing target configuration")
	}

	// Fetch the current object for the three way merge
	helper := resource.NewHelper(target.Client, target.Mapping).WithFieldManager(getManagedFieldsManager())
	currentObj, err := helper.Get(target.Namespace, target.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, types.StrategicMergePatchType, errors.Wrapf(err, "unable to get data for current object %s/%s", target.Namespace, target.Name)
	}

	// Even if currentObj is nil (because it was not found), it will marshal just fine
	currentData, err := json.Marshal(currentObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "serializing live configuration")
	}

	// Get a versioned object
	versionedObject := AsVersioned(target)

	// Unstructured objects, such as CRDs, may not have a not registered error
	// returned from ConvertToVersion. Anything that's unstructured should
	// use the jsonpatch.CreateMergePatch. Strategic Merge Patch is not supported
	// on objects like CRDs.
	_, isUnstructured := versionedObject.(runtime.Unstructured)

	// On newer K8s versions, CRDs aren't unstructured but has this dedicated type
	_, isCRD := versionedObject.(*apiextv1beta1.CustomResourceDefinition)

	if isUnstructured || isCRD {
		// fall back to generic JSON merge patch
		patch, err := jsonpatch.CreateMergePatch(oldData, newData)
		return patch, types.MergePatchType, err
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, types.StrategicMergePatchType, errors.Wrap(err, "unable to create patch metadata from object")
	}

	patch, err := strategicpatch.CreateThreeWayMergePatch(oldData, newData, currentData, patchMeta, true)
	return patch, types.StrategicMergePatchType, err
}

func updateResource(c *Client, target *resource.Info, currentObj runtime.Object, force bool) error {
	var (
		obj    runtime.Object
		helper = resource.NewHelper(target.Client, target.Mapping).WithFieldManager(getManagedFieldsManager())
		kind   = target.Mapping.GroupVersionKind.Kind
	)

	// if --force is applied, attempt to replace the existing resource with the new object.
	if force {
		var err error
		obj, err = helper.Replace(target.Namespace, target.Name, true, target.Object)
		if err != nil {
			return errors.Wrap(err, "failed to replace object")
		}
		c.Log("Replaced %q with kind %s for kind %s", target.Name, currentObj.GetObjectKind().GroupVersionKind().Kind, kind)
	} else {
		patch, patchType, err := createPatch(target, currentObj)
		if err != nil {
			return errors.Wrap(err, "failed to create patch")
		}

		if patch == nil || string(patch) == "{}" {
			c.Log("Looks like there are no changes for %s %q", kind, target.Name)
			// This needs to happen to make sure that Helm has the latest info from the API
			// Otherwise there will be no labels and other functions that use labels will panic
			if err := target.Get(); err != nil {
				return errors.Wrap(err, "failed to refresh resource information")
			}
			return nil
		}
		// send patch to server
		c.Log("Patch %s %q in namespace %s", kind, target.Name, target.Namespace)
		obj, err = helper.Patch(target.Namespace, target.Name, patchType, patch, nil)
		if err != nil {
			return errors.Wrapf(err, "cannot patch %q with kind %s", target.Name, kind)
		}
	}

	target.Refresh(obj, true)
	return nil
}

func (c *Client) watchUntilReady(timeout time.Duration, info *resource.Info) error {
	kind := info.Mapping.GroupVersionKind.Kind
	switch kind {
	case "Job", "Pod":
	default:
		return nil
	}

	c.Log("Watching for changes to %s %s with timeout of %v", kind, info.Name, timeout)

	// Use a selector on the name of the resource. This should be unique for the
	// given version and kind
	selector, err := fields.ParseSelector(fmt.Sprintf("metadata.name=%s", info.Name))
	if err != nil {
		return err
	}
	lw := cachetools.NewListWatchFromClient(info.Client, info.Mapping.Resource.Resource, info.Namespace, selector)

	// What we watch for depends on the Kind.
	// - For a Job, we watch for completion.
	// - For all else, we watch until Ready.
	// In the future, we might want to add some special logic for types
	// like Ingress, Volume, etc.

	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()
	_, err = watchtools.UntilWithSync(ctx, lw, &unstructured.Unstructured{}, nil, func(e watch.Event) (bool, error) {
		// Make sure the incoming object is versioned as we use unstructured
		// objects when we build manifests
		obj := convertWithMapper(e.Object, info.Mapping)
		switch e.Type {
		case watch.Added, watch.Modified:
			// For things like a secret or a config map, this is the best indicator
			// we get. We care mostly about jobs, where what we want to see is
			// the status go into a good state. For other types, like ReplicaSet
			// we don't really do anything to support these as hooks.
			c.Log("Add/Modify event for %s: %v", info.Name, e.Type)
			switch kind {
			case "Job":
				return c.waitForJob(obj, info.Name)
			case "Pod":
				return c.waitForPodSuccess(obj, info.Name)
			}
			return true, nil
		case watch.Deleted:
			c.Log("Deleted event for %s", info.Name)
			return true, nil
		case watch.Error:
			// Handle error and return with an error.
			c.Log("Error event for %s", info.Name)
			return true, errors.Errorf("failed to deploy %s", info.Name)
		default:
			return false, nil
		}
	})
	return err
}

// waitForJob is a helper that waits for a job to complete.
//
// This operates on an event returned from a watcher.
func (c *Client) waitForJob(obj runtime.Object, name string) (bool, error) {
	o, ok := obj.(*batch.Job)
	if !ok {
		return true, errors.Errorf("expected %s to be a *batch.Job, got %T", name, obj)
	}

	for _, c := range o.Status.Conditions {
		if c.Type == batch.JobComplete && c.Status == "True" {
			return true, nil
		} else if c.Type == batch.JobFailed && c.Status == "True" {
			return true, errors.Errorf("job %s failed: %s", name, c.Reason)
		}
	}

	c.Log("%s: Jobs active: %d, jobs failed: %d, jobs succeeded: %d", name, o.Status.Active, o.Status.Failed, o.Status.Succeeded)
	return false, nil
}

// waitForPodSuccess is a helper that waits for a pod to complete.
//
// This operates on an event returned from a watcher.
func (c *Client) waitForPodSuccess(obj runtime.Object, name string) (bool, error) {
	o, ok := obj.(*v1.Pod)
	if !ok {
		return true, errors.Errorf("expected %s to be a *v1.Pod, got %T", name, obj)
	}

	switch o.Status.Phase {
	case v1.PodSucceeded:
		c.Log("Pod %s succeeded", o.Name)
		return true, nil
	case v1.PodFailed:
		return true, errors.Errorf("pod %s failed", o.Name)
	case v1.PodPending:
		c.Log("Pod %s pending", o.Name)
	case v1.PodRunning:
		c.Log("Pod %s running", o.Name)
	}

	return false, nil
}

// scrubValidationError removes kubectl info from the message.
func scrubValidationError(err error) error {
	if err == nil {
		return nil
	}
	const stopValidateMessage = "if you choose to ignore these errors, turn validation off with --validate=false"

	if strings.Contains(err.Error(), stopValidateMessage) {
		return errors.New(strings.ReplaceAll(err.Error(), "; "+stopValidateMessage, ""))
	}
	return err
}

// WaitAndGetCompletedPodPhase waits up to a timeout until a pod enters a completed phase
// and returns said phase (PodSucceeded or PodFailed qualify).
func (c *Client) WaitAndGetCompletedPodPhase(name string, timeout time.Duration) (v1.PodPhase, error) {
	client, err := c.getKubeClient()
	if err != nil {
		return v1.PodUnknown, err
	}
	to := int64(timeout)
	watcher, err := client.CoreV1().Pods(c.namespace()).Watch(context.Background(), metav1.ListOptions{
		FieldSelector:  fmt.Sprintf("metadata.name=%s", name),
		TimeoutSeconds: &to,
	})
	if err != nil {
		return v1.PodUnknown, err
	}

	for event := range watcher.ResultChan() {
		p, ok := event.Object.(*v1.Pod)
		if !ok {
			return v1.PodUnknown, fmt.Errorf("%s not a pod", name)
		}
		switch p.Status.Phase {
		case v1.PodFailed:
			return v1.PodFailed, nil
		case v1.PodSucceeded:
			return v1.PodSucceeded, nil
		}
	}

	return v1.PodUnknown, err
}
