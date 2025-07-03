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

package kube // import "helm.sh/helm/v4/pkg/kube"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	jsonpatch "github.com/evanphx/json-patch/v5"
	v1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
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
	// Namespace allows to bypass the kubeconfig file for the choice of the namespace
	Namespace string

	Waiter
	kubeClient kubernetes.Interface
}

type WaitStrategy string

const (
	StatusWatcherStrategy WaitStrategy = "watcher"
	LegacyStrategy        WaitStrategy = "legacy"
	HookOnlyStrategy      WaitStrategy = "hookOnly"
)

type FieldValidationDirective string

const (
	FieldValidationDirectiveIgnore FieldValidationDirective = "Ignore"
	FieldValidationDirectiveWarn   FieldValidationDirective = "Warn"
	FieldValidationDirectiveStrict FieldValidationDirective = "Strict"
)

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

func (c *Client) newStatusWatcher() (*statusWaiter, error) {
	cfg, err := c.Factory.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dynamicClient, err := c.Factory.DynamicClient()
	if err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(cfg, httpClient)
	if err != nil {
		return nil, err
	}
	return &statusWaiter{
		restMapper: restMapper,
		client:     dynamicClient,
	}, nil
}

func (c *Client) GetWaiter(strategy WaitStrategy) (Waiter, error) {
	switch strategy {
	case LegacyStrategy:
		kc, err := c.Factory.KubernetesClientSet()
		if err != nil {
			return nil, err
		}
		return &legacyWaiter{kubeClient: kc}, nil
	case StatusWatcherStrategy:
		return c.newStatusWatcher()
	case HookOnlyStrategy:
		sw, err := c.newStatusWatcher()
		if err != nil {
			return nil, err
		}
		return &hookOnlyWaiter{sw: sw}, nil
	default:
		return nil, errors.New("unknown wait strategy")
	}
}

func (c *Client) SetWaiter(ws WaitStrategy) error {
	var err error
	c.Waiter, err = c.GetWaiter(ws)
	if err != nil {
		return err
	}
	return nil
}

// New creates a new Client.
func New(getter genericclioptions.RESTClientGetter) *Client {
	if getter == nil {
		getter = genericclioptions.NewConfigFlags(true)
	}
	factory := cmdutil.NewFactory(getter)
	c := &Client{
		Factory: factory,
	}
	return c
}

// getKubeClient get or create a new KubernetesClientSet
func (c *Client) getKubeClient() (kubernetes.Interface, error) {
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
		return errors.New("kubernetes cluster unreachable")
	}
	if err != nil {
		return fmt.Errorf("kubernetes cluster unreachable: %w", err)
	}
	if _, err := client.Discovery().ServerVersion(); err != nil {
		return fmt.Errorf("kubernetes cluster unreachable: %w", err)
	}
	return nil
}

type clientCreateOptions struct {
	serverSideApply          bool
	forceConflicts           bool
	dryRun                   bool
	fieldValidationDirective FieldValidationDirective
}

type ClientCreateOption func(*clientCreateOptions) error

// ClientUpdateOptionServerSideApply enables performing object apply server-side
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/
func ClientCreateOptionServerSideApply(serverSideApply bool) ClientCreateOption {
	return func(o *clientCreateOptions) error {
		o.serverSideApply = serverSideApply

		return nil
	}
}

// ClientCreateOptionForceConflicts forces field conflicts to be resolved
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
// Only valid when ClientUpdateOptionServerSideApply enabled
func ClientCreateOptionForceConflicts(forceConflicts bool) ClientCreateOption {
	return func(o *clientCreateOptions) error {
		o.forceConflicts = forceConflicts

		return nil
	}
}

// ClientCreateOptionDryRun performs non-mutating operations only
func ClientCreateOptionDryRun(dryRun bool) ClientCreateOption {
	return func(o *clientCreateOptions) error {
		o.dryRun = dryRun

		return nil
	}
}

// ClientCreateOptionFieldValidationDirective specifies show API operations validate object's schema
//   - For client-side apply: this is ignored
//   - For server-side apply: the directive is sent to the server to perform the validation
//
// Defaults to `FieldValidationDirectiveStrict`
func ClientCreateOptionFieldValidationDirective(fieldValidationDirective FieldValidationDirective) ClientCreateOption {
	return func(o *clientCreateOptions) error {
		o.fieldValidationDirective = fieldValidationDirective

		return nil
	}
}

// Create creates Kubernetes resources specified in the resource list.
func (c *Client) Create(resources ResourceList, options ...ClientCreateOption) (*Result, error) {
	slog.Debug("creating resource(s)", "resources", len(resources))

	createOptions := clientCreateOptions{
		serverSideApply:          true, // Default to server-side apply
		fieldValidationDirective: FieldValidationDirectiveStrict,
	}

	for _, o := range options {
		o(&createOptions)
	}

	if createOptions.forceConflicts && !createOptions.serverSideApply {
		return nil, fmt.Errorf("invalid operation: force conflicts can only be used with server-side apply")
	}

	makeCreateApplyFunc := func() func(target *resource.Info) error {
		if createOptions.serverSideApply {
			slog.Debug("using server-side apply for resource creation", slog.Bool("forceConflicts", createOptions.forceConflicts), slog.Bool("dryRun", createOptions.dryRun), slog.String("fieldValidationDirective", string(createOptions.fieldValidationDirective)))
			return func(target *resource.Info) error {
				err := patchResourceServerSide(target, createOptions.dryRun, createOptions.forceConflicts, createOptions.fieldValidationDirective)

				logger := slog.With(
					slog.String("namespace", target.Namespace),
					slog.String("name", target.Name),
					slog.String("gvk", target.Mapping.GroupVersionKind.String()))
				if err != nil {
					logger.Debug("Error patching resource", slog.Any("error", err))
					return err
				}

				logger.Debug("Patched resource")

				return nil
			}
		}

		slog.Debug("using client-side apply for resource creation")
		return createResource
	}

	if err := perform(resources, makeCreateApplyFunc()); err != nil {
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
					slog.Warn("get the relation pod is failed", slog.Any("error", err))
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
	slog.Debug("get relation pod of object", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind)
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

func (c *Client) namespace() string {
	if c.Namespace != "" {
		return c.Namespace
	}
	if ns, _, err := c.Factory.ToRawKubeConfigLoader().Namespace(); err == nil {
		return ns
	}
	return v1.NamespaceDefault
}

func determineFieldValidationDirective(validate bool) FieldValidationDirective {
	if validate {
		return FieldValidationDirectiveStrict
	}

	return FieldValidationDirectiveIgnore
}

func buildResourceList(f Factory, namespace string, validationDirective FieldValidationDirective, reader io.Reader, transformRequest resource.RequestTransform) (ResourceList, error) {

	schema, err := f.Validator(string(validationDirective))
	if err != nil {
		return nil, err
	}

	builder := f.NewBuilder().
		ContinueOnError().
		NamespaceParam(namespace).
		DefaultNamespace().
		Flatten().
		Unstructured().
		Schema(schema).
		Stream(reader, "")
	if transformRequest != nil {
		builder.TransformRequests(transformRequest)
	}
	result, err := builder.Do().Infos()
	return result, scrubValidationError(err)
}

// Build validates for Kubernetes objects and returns unstructured infos.
func (c *Client) Build(reader io.Reader, validate bool) (ResourceList, error) {
	return buildResourceList(
		c.Factory,
		c.namespace(),
		determineFieldValidationDirective(validate),
		reader,
		nil)
}

// BuildTable validates for Kubernetes objects and returns unstructured infos.
// The returned kind is a Table.
func (c *Client) BuildTable(reader io.Reader, validate bool) (ResourceList, error) {
	return buildResourceList(
		c.Factory,
		c.namespace(),
		determineFieldValidationDirective(validate),
		reader,
		transformRequests)
}

func (c *Client) update(target, original ResourceList, updateApplyFunc func(target, original *resource.Info) error) (*Result, error) {
	updateErrors := []error{}
	res := &Result{}

	slog.Debug("checking resources for changes", "resources", len(target))
	err := target.Visit(func(target *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(target.Client, target.Mapping).WithFieldManager(getManagedFieldsManager())
		if _, err := helper.Get(target.Namespace, target.Name); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("could not get information about the resource: %w", err)
			}

			// Append the created resource to the results, even if something fails
			res.Created = append(res.Created, target)

			// Since the resource does not exist, create it.
			if err := createResource(target); err != nil {
				return fmt.Errorf("failed to create resource: %w", err)
			}

			kind := target.Mapping.GroupVersionKind.Kind
			slog.Debug("created a new resource", "namespace", target.Namespace, "name", target.Name, "kind", kind)
			return nil
		}

		original := original.Get(target)
		if original == nil {
			kind := target.Mapping.GroupVersionKind.Kind
			return fmt.Errorf("original object %s with the name %q not found", kind, target.Name)
		}

		if err := updateApplyFunc(target, original); err != nil {
			updateErrors = append(updateErrors, err)
		}

		// Because we check for errors later, append the info regardless
		res.Updated = append(res.Updated, target)

		return nil
	})

	switch {
	case err != nil:
		return res, err
	case len(updateErrors) != 0:
		return res, joinErrors(updateErrors, " && ")
	}

	for _, info := range original.Difference(target) {
		slog.Debug("deleting resource", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind)

		if err := info.Get(); err != nil {
			slog.Debug("unable to get object", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind, slog.Any("error", err))
			continue
		}
		annotations, err := metadataAccessor.Annotations(info.Object)
		if err != nil {
			slog.Debug("unable to get annotations", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind, slog.Any("error", err))
		}
		if annotations != nil && annotations[ResourcePolicyAnno] == KeepPolicy {
			slog.Debug("skipping delete due to annotation", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind, "annotation", ResourcePolicyAnno, "value", KeepPolicy)
			continue
		}
		if err := deleteResource(info, metav1.DeletePropagationBackground); err != nil {
			slog.Debug("failed to delete resource", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind, slog.Any("error", err))
			continue
		}
		res.Deleted = append(res.Deleted, info)
	}
	return res, nil
}

type clientUpdateOptions struct {
	threeWayMergeForUnstructured bool
	serverSideApply              bool
	forceReplace                 bool
	forceConflicts               bool
	dryRun                       bool
	fieldValidationDirective     FieldValidationDirective
}

type ClientUpdateOption func(*clientUpdateOptions) error

// ClientUpdateOptionThreeWayMergeForUnstructured enables performing three-way merge for unstructured objects
// Must not be enabled when ClientUpdateOptionServerSideApply is enabled
func ClientUpdateOptionThreeWayMergeForUnstructured(threeWayMergeForUnstructured bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.threeWayMergeForUnstructured = threeWayMergeForUnstructured

		return nil
	}
}

// ClientUpdateOptionServerSideApply enables performing object apply server-side (default)
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/
// Must not be enabled when ClientUpdateOptionThreeWayMerge is enabled
func ClientUpdateOptionServerSideApply(serverSideApply bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.serverSideApply = serverSideApply

		return nil
	}
}

// ClientUpdateOptionForceReplace forces objects to be replaced rather than updated via patch
// Must not be enabled when ClientUpdateOptionForceConflicts is enabled
func ClientUpdateOptionForceReplace(forceReplace bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.forceReplace = forceReplace

		return nil
	}
}

// ClientUpdateOptionForceConflicts forces field conflicts to be resolved
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
// Must not be enabled when ClientUpdateOptionForceReplace is enabled
func ClientUpdateOptionForceConflicts(forceConflicts bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.forceConflicts = forceConflicts

		return nil
	}
}

// ClientUpdateOptionForceConflicts forces field conflicts to be resolved
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
// Must not be enabled when ClientUpdateOptionForceReplace is enabled
func ClientUpdateOptionDryRun(dryRun bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.dryRun = dryRun

		return nil
	}
}

// ClientUpdateOptionFieldValidationDirective specifies show API operations validate object's schema
//   - For client-side apply: this is ignored
//   - For server-side apply: the directive is sent to the server to perform the validation
//
// Defaults to `FieldValidationDirectiveStrict`
func ClientUpdateOptionFieldValidationDirective(fieldValidationDirective FieldValidationDirective) ClientCreateOption {
	return func(o *clientCreateOptions) error {
		o.fieldValidationDirective = fieldValidationDirective

		return nil
	}
}

// Update takes the current list of objects and target list of objects and
// creates resources that don't already exist, updates resources that have been
// modified in the target configuration, and deletes resources from the current
// configuration that are not present in the target configuration. If an error
// occurs, a Result will still be returned with the error, containing all
// resource updates, creations, and deletions that were attempted. These can be
// used for cleanup or other logging purposes.
//
// The default is to use server-side apply, equivalent to: `ClientUpdateOptionServerSideApply(true)`
func (c *Client) Update(original, target ResourceList, options ...ClientUpdateOption) (*Result, error) {
	updateOptions := clientUpdateOptions{
		serverSideApply:          true, // Default to server-side apply
		fieldValidationDirective: FieldValidationDirectiveStrict,
	}

	for _, o := range options {
		o(&updateOptions)
	}

	if updateOptions.threeWayMergeForUnstructured && updateOptions.serverSideApply {
		return nil, fmt.Errorf("invalid operation: cannot use three-way merge for unstructured and server-side apply together")
	}

	if updateOptions.forceConflicts && updateOptions.forceReplace {
		return nil, fmt.Errorf("invalid operation: cannot use force conflicts and force replace together")
	}

	if updateOptions.serverSideApply && updateOptions.forceReplace {
		return nil, fmt.Errorf("invalid operation: cannot use server-side apply and force replace together")
	}

	makeUpdateApplyFunc := func() func(target, original *resource.Info) error {
		if updateOptions.forceReplace {
			slog.Debug(
				"using resource replace update strategy",
				slog.String("fieldValidationDirective", string(updateOptions.fieldValidationDirective)))
			return func(target, original *resource.Info) error {
				if err := replaceResource(target, updateOptions.fieldValidationDirective); err != nil {
					slog.Debug("error replacing the resource", "namespace", target.Namespace, "name", target.Name, "kind", target.Mapping.GroupVersionKind.Kind, slog.Any("error", err))
					return err
				}

				originalObject := original.Object
				kind := target.Mapping.GroupVersionKind.Kind
				slog.Debug("replace succeeded", "name", original.Name, "initialKind", originalObject.GetObjectKind().GroupVersionKind().Kind, "kind", kind)

				return nil
			}
		} else if updateOptions.serverSideApply {
			slog.Debug(
				"using server-side apply for resource update",
				slog.Bool("forceConflicts", updateOptions.forceConflicts),
				slog.Bool("dryRun", updateOptions.dryRun),
				slog.String("fieldValidationDirective", string(updateOptions.fieldValidationDirective)))
			return func(target, _ *resource.Info) error {
				err := patchResourceServerSide(target, updateOptions.dryRun, updateOptions.forceConflicts, updateOptions.fieldValidationDirective)

				logger := slog.With(
					slog.String("namespace", target.Namespace),
					slog.String("name", target.Name),
					slog.String("gvk", target.Mapping.GroupVersionKind.String()))
				if err != nil {
					logger.Debug("Error patching resource", slog.Any("error", err))
					return err
				}

				logger.Debug("Patched resource")

				return nil
			}
		}

		slog.Debug("using client-side apply for resource update", slog.Bool("threeWayMergeForUnstructured", updateOptions.threeWayMergeForUnstructured))
		return func(target, original *resource.Info) error {
			return patchResourceClientSide(target, original.Object, updateOptions.threeWayMergeForUnstructured)
		}
	}

	return c.update(target, original, makeUpdateApplyFunc())
}

// Delete deletes Kubernetes resources specified in the resources list with
// background cascade deletion. It will attempt to delete all resources even
// if one or more fail and collect any errors. All successfully deleted items
// will be returned in the `Deleted` ResourceList that is part of the result.
func (c *Client) Delete(resources ResourceList) (*Result, []error) {
	return deleteResources(resources, metav1.DeletePropagationBackground)
}

// Delete deletes Kubernetes resources specified in the resources list with
// given deletion propagation policy. It will attempt to delete all resources even
// if one or more fail and collect any errors. All successfully deleted items
// will be returned in the `Deleted` ResourceList that is part of the result.
func (c *Client) DeleteWithPropagationPolicy(resources ResourceList, policy metav1.DeletionPropagation) (*Result, []error) {
	return deleteResources(resources, policy)
}

func deleteResources(resources ResourceList, propagation metav1.DeletionPropagation) (*Result, []error) {
	var errs []error
	res := &Result{}
	mtx := sync.Mutex{}
	err := perform(resources, func(info *resource.Info) error {
		slog.Debug("starting delete resource", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind)
		err := deleteResource(info, propagation)
		if err == nil || apierrors.IsNotFound(err) {
			if err != nil {
				slog.Debug("ignoring delete failure", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind, slog.Any("error", err))
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

// https://github.com/kubernetes/kubectl/blob/197123726db24c61aa0f78d1f0ba6e91a2ec2f35/pkg/cmd/apply/apply.go#L439
func isIncompatibleServerError(err error) bool {
	// 415: Unsupported media type means we're talking to a server which doesn't
	// support server-side apply.
	if _, ok := err.(*apierrors.StatusError); !ok {
		// Non-StatusError means the error isn't because the server is incompatible.
		return false
	}
	return err.(*apierrors.StatusError).Status().Code == http.StatusUnsupportedMediaType
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
			result = errors.Join(result, err)
		}
	}

	return result
}

func batchPerform(infos ResourceList, fn func(*resource.Info) error, errs chan<- error) {
	var kind string
	var wg sync.WaitGroup
	defer wg.Wait()

	for _, info := range infos {
		currentKind := info.Object.GetObjectKind().GroupVersionKind().Kind
		if kind != currentKind {
			wg.Wait()
			kind = currentKind
		}

		wg.Add(1)
		go func(info *resource.Info) {
			errs <- fn(info)
			wg.Done()
		}(info)
	}
}

var createMutex sync.Mutex

func createResource(info *resource.Info) error {
	return retry.RetryOnConflict(
		retry.DefaultRetry,
		func() error {
			createMutex.Lock()
			defer createMutex.Unlock()
			obj, err := resource.NewHelper(info.Client, info.Mapping).WithFieldManager(getManagedFieldsManager()).Create(info.Namespace, true, info.Object)
			if err != nil {
				return err
			}

			return info.Refresh(obj, true)
		})
}

func deleteResource(info *resource.Info, policy metav1.DeletionPropagation) error {
	return retry.RetryOnConflict(
		retry.DefaultRetry,
		func() error {
			opts := &metav1.DeleteOptions{PropagationPolicy: &policy}
			_, err := resource.NewHelper(info.Client, info.Mapping).WithFieldManager(getManagedFieldsManager()).DeleteWithOptions(info.Namespace, info.Name, opts)
			return err
		})
}

func createPatch(target *resource.Info, current runtime.Object, threeWayMergeForUnstructured bool) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(current)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing current configuration: %w", err)
	}
	newData, err := json.Marshal(target.Object)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing target configuration: %w", err)
	}

	// Fetch the current object for the three way merge
	helper := resource.NewHelper(target.Client, target.Mapping).WithFieldManager(getManagedFieldsManager())
	currentObj, err := helper.Get(target.Namespace, target.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, types.StrategicMergePatchType, fmt.Errorf("unable to get data for current object %s/%s: %w", target.Namespace, target.Name, err)
	}

	// Even if currentObj is nil (because it was not found), it will marshal just fine
	currentData, err := json.Marshal(currentObj)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("serializing live configuration: %w", err)
	}

	// Get a versioned object
	versionedObject := AsVersioned(target)

	// Unstructured objects, such as CRDs, may not have a not registered error
	// returned from ConvertToVersion. Anything that's unstructured should
	// use generic JSON merge patch. Strategic Merge Patch is not supported
	// on objects like CRDs.
	_, isUnstructured := versionedObject.(runtime.Unstructured)

	// On newer K8s versions, CRDs aren't unstructured but has this dedicated type
	_, isCRD := versionedObject.(*apiextv1beta1.CustomResourceDefinition)

	if isUnstructured || isCRD {
		if threeWayMergeForUnstructured {
			// from https://github.com/kubernetes/kubectl/blob/b83b2ec7d15f286720bccf7872b5c72372cb8e80/pkg/cmd/apply/patcher.go#L129
			preconditions := []mergepatch.PreconditionFunc{
				mergepatch.RequireKeyUnchanged("apiVersion"),
				mergepatch.RequireKeyUnchanged("kind"),
				mergepatch.RequireMetadataKeyUnchanged("name"),
			}
			patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(oldData, newData, currentData, preconditions...)
			if err != nil && mergepatch.IsPreconditionFailed(err) {
				err = fmt.Errorf("%w: at least one field was changed: apiVersion, kind or name", err)
			}
			return patch, types.MergePatchType, err
		}
		// fall back to generic JSON merge patch
		patch, err := jsonpatch.CreateMergePatch(oldData, newData)
		return patch, types.MergePatchType, err
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, types.StrategicMergePatchType, fmt.Errorf("unable to create patch metadata from object: %w", err)
	}

	patch, err := strategicpatch.CreateThreeWayMergePatch(oldData, newData, currentData, patchMeta, true)
	return patch, types.StrategicMergePatchType, err
}

func replaceResource(target *resource.Info, fieldValidationDirective FieldValidationDirective) error {

	helper := resource.NewHelper(target.Client, target.Mapping).
		WithFieldValidation(string(fieldValidationDirective)).
		WithFieldManager(getManagedFieldsManager())

	obj, err := helper.Replace(target.Namespace, target.Name, true, target.Object)
	if err != nil {
		return fmt.Errorf("failed to replace object: %w", err)
	}

	if err := target.Refresh(obj, true); err != nil {
		return fmt.Errorf("failed to refresh object after replace: %w", err)
	}

	return nil

}

func patchResourceClientSide(target *resource.Info, original runtime.Object, threeWayMergeForUnstructured bool) error {

	patch, patchType, err := createPatch(target, original, threeWayMergeForUnstructured)
	if err != nil {
		return fmt.Errorf("failed to create patch: %w", err)
	}

	kind := target.Mapping.GroupVersionKind.Kind
	if patch == nil || string(patch) == "{}" {
		slog.Debug("no changes detected", "kind", kind, "name", target.Name)
		// This needs to happen to make sure that Helm has the latest info from the API
		// Otherwise there will be no labels and other functions that use labels will panic
		if err := target.Get(); err != nil {
			return fmt.Errorf("failed to refresh resource information: %w", err)
		}
		return nil
	}

	// send patch to server
	slog.Debug("patching resource", "kind", kind, "name", target.Name, "namespace", target.Namespace)
	helper := resource.NewHelper(target.Client, target.Mapping).WithFieldManager(getManagedFieldsManager())
	obj, err := helper.Patch(target.Namespace, target.Name, patchType, patch, nil)
	if err != nil {
		return fmt.Errorf("cannot patch %q with kind %s: %w", target.Name, kind, err)
	}

	target.Refresh(obj, true)

	return nil
}

// Patch reource using server-side apply
func patchResourceServerSide(info *resource.Info, dryRun bool, forceConflicts bool, fieldValidationDirective FieldValidationDirective) error {
	helper := resource.NewHelper(
		info.Client,
		info.Mapping).
		DryRun(dryRun).
		WithFieldManager(ManagedFieldsManager).
		WithFieldValidation(string(fieldValidationDirective))

	// Send the full object to be applied on the server side.
	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, info.Object)
	if err != nil {
		return fmt.Errorf("failed to encode object %s/%s with kind %s: %w", info.Namespace, info.Name, info.Mapping.GroupVersionKind.Kind, err)
	}
	options := metav1.PatchOptions{
		Force: &forceConflicts,
	}
	obj, err := helper.Patch(
		info.Namespace,
		info.Name,
		types.ApplyPatchType,
		data,
		&options,
	)
	if err != nil {
		if isIncompatibleServerError(err) {
			return fmt.Errorf("server-side apply not available on the server: %v", err)
		}

		if apierrors.IsConflict(err) {
			return fmt.Errorf("conflict occurred while applying %s/%s with kind %s: %w", info.Namespace, info.Name, info.Mapping.GroupVersionKind.Kind, err)
		}

		return err
	}

	return info.Refresh(obj, true)
}

// GetPodList uses the kubernetes interface to get the list of pods filtered by listOptions
func (c *Client) GetPodList(namespace string, listOptions metav1.ListOptions) (*v1.PodList, error) {
	podList, err := c.kubeClient.CoreV1().Pods(namespace).List(context.Background(), listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod list with options: %+v with error: %v", listOptions, err)
	}
	return podList, nil
}

// OutputContainerLogsForPodList is a helper that outputs logs for a list of pods
func (c *Client) OutputContainerLogsForPodList(podList *v1.PodList, namespace string, writerFunc func(namespace, pod, container string) io.Writer) error {
	for _, pod := range podList.Items {
		for _, container := range pod.Spec.Containers {
			options := &v1.PodLogOptions{
				Container: container.Name,
			}
			request := c.kubeClient.CoreV1().Pods(namespace).GetLogs(pod.Name, options)
			err2 := copyRequestStreamToWriter(request, pod.Name, container.Name, writerFunc(namespace, pod.Name, container.Name))
			if err2 != nil {
				return err2
			}
		}
	}
	return nil
}

func copyRequestStreamToWriter(request *rest.Request, podName, containerName string, writer io.Writer) error {
	readCloser, err := request.Stream(context.Background())
	if err != nil {
		return fmt.Errorf("failed to stream pod logs for pod: %s, container: %s", podName, containerName)
	}
	defer readCloser.Close()
	_, err = io.Copy(writer, readCloser)
	if err != nil {
		return fmt.Errorf("failed to copy IO from logs for pod: %s, container: %s", podName, containerName)
	}
	return nil
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

type joinedErrors struct {
	errs []error
	sep  string
}

func joinErrors(errs []error, sep string) error {
	return &joinedErrors{
		errs: errs,
		sep:  sep,
	}
}

func (e *joinedErrors) Error() string {
	errs := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		errs = append(errs, err.Error())
	}
	return strings.Join(errs, e.sep)
}

func (e *joinedErrors) Unwrap() []error {
	return e.errs
}
