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

	"helm.sh/helm/v4/internal/logging"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/csaupgrade"
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

	// WaitContext is an optional context to use for wait operations.
	// If not set, a context will be created internally using the
	// timeout provided to the wait functions.
	//
	// Deprecated: Use WithWaitContext wait option when getting a Waiter instead.
	WaitContext context.Context

	Waiter
	kubeClient kubernetes.Interface

	// Embed a LogHolder to provide logger functionality
	logging.LogHolder
}

var _ Interface = (*Client)(nil)

// WaitStrategy represents the algorithm used to wait for Kubernetes
// resources to reach their desired state.
type WaitStrategy string

const (
	// StatusWatcherStrategy: event-driven waits using kstatus (watches + aggregated readers).
	// Default for --wait. More accurate and responsive; waits CRs and full reconciliation.
	// Requires: reachable API server, list+watch RBAC on deployed resources, and a non-zero timeout.
	StatusWatcherStrategy WaitStrategy = "watcher"

	// LegacyStrategy: Helm 3-style periodic polling until ready or timeout.
	// Use when watches aren’t available/reliable, or for compatibility/simple CI.
	// Requires only list RBAC for polled resources.
	LegacyStrategy WaitStrategy = "legacy"

	// HookOnlyStrategy: wait only for hook Pods/Jobs to complete; does not wait for general chart resources.
	HookOnlyStrategy WaitStrategy = "hookOnly"
)

type FieldValidationDirective string

const (
	FieldValidationDirectiveIgnore FieldValidationDirective = "Ignore"
	FieldValidationDirectiveWarn   FieldValidationDirective = "Warn"
	FieldValidationDirectiveStrict FieldValidationDirective = "Strict"
)

type CreateApplyFunc func(target *resource.Info) error
type UpdateApplyFunc func(original, target *resource.Info) error

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

func (c *Client) newStatusWatcher(opts ...WaitOption) (*statusWaiter, error) {
	var o waitOptions
	for _, opt := range opts {
		opt(&o)
	}
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
	waitContext := o.ctx
	if waitContext == nil {
		waitContext = c.WaitContext
	}
	return &statusWaiter{
		restMapper:         restMapper,
		client:             dynamicClient,
		ctx:                waitContext,
		watchUntilReadyCtx: o.watchUntilReadyCtx,
		waitCtx:            o.waitCtx,
		waitWithJobsCtx:    o.waitWithJobsCtx,
		waitForDeleteCtx:   o.waitForDeleteCtx,
		readers:            o.statusReaders,
	}, nil
}

func (c *Client) GetWaiter(ws WaitStrategy) (Waiter, error) {
	return c.GetWaiterWithOptions(ws)
}

func (c *Client) GetWaiterWithOptions(strategy WaitStrategy, opts ...WaitOption) (Waiter, error) {
	switch strategy {
	case LegacyStrategy:
		kc, err := c.Factory.KubernetesClientSet()
		if err != nil {
			return nil, err
		}
		return &legacyWaiter{kubeClient: kc, ctx: c.WaitContext}, nil
	case StatusWatcherStrategy:
		return c.newStatusWatcher(opts...)
	case HookOnlyStrategy:
		sw, err := c.newStatusWatcher(opts...)
		if err != nil {
			return nil, err
		}
		return &hookOnlyWaiter{sw: sw}, nil
	case "":
		return nil, errors.New("wait strategy not set. Choose one of: " + string(StatusWatcherStrategy) + ", " + string(HookOnlyStrategy) + ", " + string(LegacyStrategy))
	default:
		return nil, errors.New("unknown wait strategy (s" + string(strategy) + "). Valid values are: " + string(StatusWatcherStrategy) + ", " + string(HookOnlyStrategy) + ", " + string(LegacyStrategy))
	}
}

func (c *Client) SetWaiter(ws WaitStrategy) error {
	return c.SetWaiterWithOptions(ws)
}

func (c *Client) SetWaiterWithOptions(ws WaitStrategy, opts ...WaitOption) error {
	var err error
	c.Waiter, err = c.GetWaiterWithOptions(ws, opts...)
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
	c.SetLogger(slog.Default().Handler())
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

// ClientCreateOptionServerSideApply enables performing object apply server-side
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/
//
// `forceConflicts` forces conflicts to be resolved (may be  when serverSideApply enabled only)
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
func ClientCreateOptionServerSideApply(serverSideApply, forceConflicts bool) ClientCreateOption {
	return func(o *clientCreateOptions) error {
		if !serverSideApply && forceConflicts {
			return fmt.Errorf("forceConflicts enabled when serverSideApply disabled")
		}

		o.serverSideApply = serverSideApply
		o.forceConflicts = forceConflicts

		return nil
	}
}

// ClientCreateOptionDryRun requests the server to perform non-mutating operations only
func ClientCreateOptionDryRun(dryRun bool) ClientCreateOption {
	return func(o *clientCreateOptions) error {
		o.dryRun = dryRun

		return nil
	}
}

// ClientCreateOptionFieldValidationDirective specifies how API operations validate object's schema
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

func (c *Client) makeCreateApplyFunc(serverSideApply, forceConflicts, dryRun bool, fieldValidationDirective FieldValidationDirective) CreateApplyFunc {
	if serverSideApply {
		c.Logger().Debug(
			"using server-side apply for resource creation",
			slog.Bool("forceConflicts", forceConflicts),
			slog.Bool("dryRun", dryRun),
			slog.String("fieldValidationDirective", string(fieldValidationDirective)))

		return func(target *resource.Info) error {
			err := patchResourceServerSide(target, dryRun, forceConflicts, fieldValidationDirective)

			logger := c.Logger().With(
				slog.String("namespace", target.Namespace),
				slog.String("name", target.Name),
				slog.String("gvk", target.Mapping.GroupVersionKind.String()))
			if err != nil {
				logger.Debug("Error creating resource via patch", slog.Any("error", err))
				return err
			}

			logger.Debug("Created resource via patch")

			return nil
		}
	}

	c.Logger().Debug("using client-side apply for resource creation")
	return createResource
}

// Create creates Kubernetes resources specified in the resource list.
func (c *Client) Create(resources ResourceList, options ...ClientCreateOption) (*Result, error) {
	c.Logger().Debug("creating resource(s)", "resources", len(resources))

	createOptions := clientCreateOptions{
		serverSideApply:          true, // Default to server-side apply
		fieldValidationDirective: FieldValidationDirectiveStrict,
	}

	errs := make([]error, 0, len(options))
	for _, o := range options {
		errs = append(errs, o(&createOptions))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("invalid client create option(s): %w", err)
	}

	createApplyFunc := c.makeCreateApplyFunc(
		createOptions.serverSideApply,
		createOptions.forceConflicts,
		createOptions.dryRun,
		createOptions.fieldValidationDirective)
	if err := perform(resources, createApplyFunc); err != nil {
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
					c.Logger().Warn("get the relation pod is failed", slog.Any("error", err))
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
	c.Logger().Debug("get relation pod of object", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind)
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

func (c *Client) update(originals, targets ResourceList, createApplyFunc CreateApplyFunc, updateApplyFunc UpdateApplyFunc) (*Result, error) {
	updateErrors := []error{}
	res := &Result{}

	c.Logger().Debug("checking resources for changes", "resources", len(targets))
	err := targets.Visit(func(target *resource.Info, err error) error {
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
			if err := createApplyFunc(target); err != nil {
				return fmt.Errorf("failed to create resource: %w", err)
			}

			kind := target.Mapping.GroupVersionKind.Kind
			c.Logger().Debug(
				"created a new resource",
				slog.String("namespace", target.Namespace),
				slog.String("name", target.Name),
				slog.String("kind", kind),
			)
			return nil
		}

		original := originals.Get(target)
		if original == nil {
			kind := target.Mapping.GroupVersionKind.Kind
			return fmt.Errorf("original object %s with the name %q not found", kind, target.Name)
		}

		if err := updateApplyFunc(original, target); err != nil {
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

	for _, info := range originals.Difference(targets) {
		c.Logger().Debug("deleting resource", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind)

		if err := info.Get(); err != nil {
			c.Logger().Debug(
				"unable to get object",
				slog.String("namespace", info.Namespace),
				slog.String("name", info.Name),
				slog.String("kind", info.Mapping.GroupVersionKind.Kind),
				slog.Any("error", err),
			)
			continue
		}
		annotations, err := metadataAccessor.Annotations(info.Object)
		if err != nil {
			c.Logger().Debug(
				"unable to get annotations",
				slog.String("namespace", info.Namespace),
				slog.String("name", info.Name),
				slog.String("kind", info.Mapping.GroupVersionKind.Kind),
				slog.Any("error", err),
			)
		}
		if annotations != nil && annotations[ResourcePolicyAnno] == KeepPolicy {
			c.Logger().Debug("skipping delete due to annotation", "namespace", info.Namespace, "name", info.Name, "kind", info.Mapping.GroupVersionKind.Kind, "annotation", ResourcePolicyAnno, "value", KeepPolicy)
			continue
		}
		if err := deleteResource(info, metav1.DeletePropagationBackground); err != nil {
			c.Logger().Debug(
				"failed to delete resource",
				slog.String("namespace", info.Namespace),
				slog.String("name", info.Name),
				slog.String("kind", info.Mapping.GroupVersionKind.Kind),
				slog.Any("error", err),
			)
			if !apierrors.IsNotFound(err) {
				updateErrors = append(updateErrors, fmt.Errorf("failed to delete resource %s: %w", info.Name, err))
			}
			continue
		}
		res.Deleted = append(res.Deleted, info)
	}

	if len(updateErrors) != 0 {
		return res, joinErrors(updateErrors, " && ")
	}
	return res, nil
}

type clientUpdateOptions struct {
	threeWayMergeForUnstructured  bool
	serverSideApply               bool
	forceReplace                  bool
	forceConflicts                bool
	dryRun                        bool
	fieldValidationDirective      FieldValidationDirective
	upgradeClientSideFieldManager bool
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
//
// `forceConflicts` forces conflicts to be resolved (may be enabled when serverSideApply enabled only)
// see: https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
func ClientUpdateOptionServerSideApply(serverSideApply, forceConflicts bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		if !serverSideApply && forceConflicts {
			return fmt.Errorf("forceConflicts enabled when serverSideApply disabled")
		}

		o.serverSideApply = serverSideApply
		o.forceConflicts = forceConflicts

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

// ClientUpdateOptionDryRun requests the server to perform non-mutating operations only
func ClientUpdateOptionDryRun(dryRun bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.dryRun = dryRun

		return nil
	}
}

// ClientUpdateOptionFieldValidationDirective specifies how API operations validate object's schema
//   - For client-side apply: this is ignored
//   - For server-side apply: the directive is sent to the server to perform the validation
//
// Defaults to `FieldValidationDirectiveStrict`
func ClientUpdateOptionFieldValidationDirective(fieldValidationDirective FieldValidationDirective) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.fieldValidationDirective = fieldValidationDirective

		return nil
	}
}

// ClientUpdateOptionUpgradeClientSideFieldManager specifies that resources client-side field manager should be upgraded to server-side apply
// (before applying the object server-side)
// This is required when upgrading a chart from client-side to server-side apply, otherwise the client-side field management remains. Conflicting with server-side applied updates.
//
// Note:
// if this option is specified, but the object is not managed by client-side field manager, it will be a no-op. However, the cost of fetching the objects will be incurred.
//
// see:
// - https://github.com/kubernetes/kubernetes/pull/112905
// - `UpgradeManagedFields` / https://github.com/kubernetes/kubernetes/blob/f47e9696d7237f1011d23c9b55f6947e60526179/staging/src/k8s.io/client-go/util/csaupgrade/upgrade.go#L81
func ClientUpdateOptionUpgradeClientSideFieldManager(upgradeClientSideFieldManager bool) ClientUpdateOption {
	return func(o *clientUpdateOptions) error {
		o.upgradeClientSideFieldManager = upgradeClientSideFieldManager

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
func (c *Client) Update(originals, targets ResourceList, options ...ClientUpdateOption) (*Result, error) {
	updateOptions := clientUpdateOptions{
		serverSideApply:          true, // Default to server-side apply
		fieldValidationDirective: FieldValidationDirectiveStrict,
	}

	errs := make([]error, 0, len(options))
	for _, o := range options {
		errs = append(errs, o(&updateOptions))
	}
	if err := errors.Join(errs...); err != nil {
		return &Result{}, fmt.Errorf("invalid client update option(s): %w", err)
	}

	if updateOptions.threeWayMergeForUnstructured && updateOptions.serverSideApply {
		return &Result{}, fmt.Errorf("invalid operation: cannot use three-way merge for unstructured and server-side apply together")
	}

	if updateOptions.forceConflicts && updateOptions.forceReplace {
		return &Result{}, fmt.Errorf("invalid operation: cannot use force conflicts and force replace together")
	}

	if updateOptions.serverSideApply && updateOptions.forceReplace {
		return &Result{}, fmt.Errorf("invalid operation: cannot use server-side apply and force replace together")
	}

	createApplyFunc := c.makeCreateApplyFunc(
		updateOptions.serverSideApply,
		updateOptions.forceConflicts,
		updateOptions.dryRun,
		updateOptions.fieldValidationDirective)

	makeUpdateApplyFunc := func() UpdateApplyFunc {
		if updateOptions.forceReplace {
			c.Logger().Debug(
				"using resource replace update strategy",
				slog.String("fieldValidationDirective", string(updateOptions.fieldValidationDirective)))
			return func(original, target *resource.Info) error {
				if err := replaceResource(target, updateOptions.fieldValidationDirective); err != nil {
					c.Logger().With(
						slog.String("namespace", target.Namespace),
						slog.String("name", target.Name),
						slog.String("gvk", target.Mapping.GroupVersionKind.String()),
					).Debug(
						"error replacing the resource", slog.Any("error", err),
					)
					return err
				}

				originalObject := original.Object
				kind := target.Mapping.GroupVersionKind.Kind
				c.Logger().Debug("replace succeeded", "name", original.Name, "initialKind", originalObject.GetObjectKind().GroupVersionKind().Kind, "kind", kind)

				return nil
			}
		} else if updateOptions.serverSideApply {
			c.Logger().Debug(
				"using server-side apply for resource update",
				slog.Bool("forceConflicts", updateOptions.forceConflicts),
				slog.Bool("dryRun", updateOptions.dryRun),
				slog.String("fieldValidationDirective", string(updateOptions.fieldValidationDirective)),
				slog.Bool("upgradeClientSideFieldManager", updateOptions.upgradeClientSideFieldManager))
			return func(original, target *resource.Info) error {

				logger := c.Logger().With(
					slog.String("namespace", target.Namespace),
					slog.String("name", target.Name),
					slog.String("gvk", target.Mapping.GroupVersionKind.String()))

				if updateOptions.upgradeClientSideFieldManager {
					patched, err := upgradeClientSideFieldManager(original, updateOptions.dryRun, updateOptions.fieldValidationDirective)
					if err != nil {
						c.Logger().Debug("Error patching resource to replace CSA field management", slog.Any("error", err))
						return err
					}

					if patched {
						logger.Debug("Upgraded object client-side field management with server-side apply field management")
					}
				}

				if err := patchResourceServerSide(target, updateOptions.dryRun, updateOptions.forceConflicts, updateOptions.fieldValidationDirective); err != nil {
					logger.Debug("Error patching resource", slog.Any("error", err))
					return err
				}

				logger.Debug("Patched resource")

				return nil
			}
		}

		c.Logger().Debug("using client-side apply for resource update", slog.Bool("threeWayMergeForUnstructured", updateOptions.threeWayMergeForUnstructured))
		return func(original, target *resource.Info) error {
			return patchResourceClientSide(original.Object, target, updateOptions.threeWayMergeForUnstructured)
		}
	}

	return c.update(originals, targets, createApplyFunc, makeUpdateApplyFunc())
}

// Delete deletes Kubernetes resources specified in the resources list with
// given deletion propagation policy. It will attempt to delete all resources even
// if one or more fail and collect any errors. All successfully deleted items
// will be returned in the `Deleted` ResourceList that is part of the result.
func (c *Client) Delete(resources ResourceList, policy metav1.DeletionPropagation) (*Result, []error) {
	var errs []error
	res := &Result{}
	mtx := sync.Mutex{}
	err := perform(resources, func(target *resource.Info) error {
		c.Logger().Debug("starting delete resource", "namespace", target.Namespace, "name", target.Name, "kind", target.Mapping.GroupVersionKind.Kind)
		err := deleteResource(target, policy)
		if err == nil || apierrors.IsNotFound(err) {
			if err != nil {
				c.Logger().Debug(
					"ignoring delete failure",
					slog.String("namespace", target.Namespace),
					slog.String("name", target.Name),
					slog.String("kind", target.Mapping.GroupVersionKind.Kind),
					slog.Any("error", err))
			}
			mtx.Lock()
			defer mtx.Unlock()
			res.Deleted = append(res.Deleted, target)
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

func createPatch(original runtime.Object, target *resource.Info, threeWayMergeForUnstructured bool) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(original)
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

func patchResourceClientSide(original runtime.Object, target *resource.Info, threeWayMergeForUnstructured bool) error {

	patch, patchType, err := createPatch(original, target, threeWayMergeForUnstructured)
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

// upgradeClientSideFieldManager is simply a wrapper around csaupgrade.UpgradeManagedFields
// that upgrade CSA managed fields to SSA apply
// see: https://github.com/kubernetes/kubernetes/pull/112905
func upgradeClientSideFieldManager(info *resource.Info, dryRun bool, fieldValidationDirective FieldValidationDirective) (bool, error) {

	fieldManagerName := getManagedFieldsManager()

	patched := false
	err := retry.RetryOnConflict(
		retry.DefaultRetry,
		func() error {

			if err := info.Get(); err != nil {
				return fmt.Errorf("failed to get object %s/%s %s: %w", info.Namespace, info.Name, info.Mapping.GroupVersionKind.String(), err)
			}

			helper := resource.NewHelper(
				info.Client,
				info.Mapping).
				DryRun(dryRun).
				WithFieldManager(fieldManagerName).
				WithFieldValidation(string(fieldValidationDirective))

			patchData, err := csaupgrade.UpgradeManagedFieldsPatch(
				info.Object,
				sets.New(fieldManagerName),
				fieldManagerName)
			if err != nil {
				return fmt.Errorf("failed to upgrade managed fields for object %s/%s %s: %w", info.Namespace, info.Name, info.Mapping.GroupVersionKind.String(), err)
			}

			if len(patchData) == 0 {
				return nil
			}

			obj, err := helper.Patch(
				info.Namespace,
				info.Name,
				types.JSONPatchType,
				patchData,
				nil)

			if err == nil {
				patched = true
				return info.Refresh(obj, true)
			}

			if !apierrors.IsConflict(err) {
				return fmt.Errorf("failed to patch object to upgrade CSA field manager %s/%s %s: %w", info.Namespace, info.Name, info.Mapping.GroupVersionKind.String(), err)
			}

			return err
		})

	return patched, err
}

// Patch reource using server-side apply
func patchResourceServerSide(target *resource.Info, dryRun bool, forceConflicts bool, fieldValidationDirective FieldValidationDirective) error {
	helper := resource.NewHelper(
		target.Client,
		target.Mapping).
		DryRun(dryRun).
		WithFieldManager(getManagedFieldsManager()).
		WithFieldValidation(string(fieldValidationDirective))

	// Send the full object to be applied on the server side.
	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, target.Object)
	if err != nil {
		return fmt.Errorf("failed to encode object %s/%s %s: %w", target.Namespace, target.Name, target.Mapping.GroupVersionKind.String(), err)
	}
	options := metav1.PatchOptions{
		Force: &forceConflicts,
	}
	obj, err := helper.Patch(
		target.Namespace,
		target.Name,
		types.ApplyPatchType,
		data,
		&options,
	)
	if err != nil {
		if isIncompatibleServerError(err) {
			return fmt.Errorf("server-side apply not available on the server: %v", err)
		}

		if apierrors.IsConflict(err) {
			return fmt.Errorf("conflict occurred while applying object %s/%s %s: %w", target.Namespace, target.Name, target.Mapping.GroupVersionKind.String(), err)
		}

		return err
	}

	return target.Refresh(obj, true)
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
