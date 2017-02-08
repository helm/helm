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

package helm // import "k8s.io/helm/pkg/helm"

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/technosophos/moniker"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"gopkg.in/square/go-jose.v1/json"
	hapi "k8s.io/helm/api"
	cs "k8s.io/helm/client/clientset"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/kube"
	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	rs "k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	rest "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"log"
	"math/rand"
	"strconv"
	"time"
)

// Client manages client side of the helm-tiller protocol
func init() {
	rand.Seed(time.Now().UnixNano())
}

type Client struct {
	opts options
}

type ExtensionsClient struct {
	restClient rest.Interface
}

// NewClient creates a new client.
func NewClient(opts ...Option) *Client {
	var c Client
	return c.Option(opts...)
}

// Option configures the helm client with the provided options
func (h *Client) Option(opts ...Option) *Client {
	for _, opt := range opts {
		opt(&h.opts)
	}
	return h
}

// ListReleases lists the current releases.
func (h *Client) ListReleases(opts ...ReleaseListOption) (*rls.ListReleasesResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.listReq
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.list(ctx, req)
}

// InstallRelease loads a chart from chstr, installs it and returns the release response.
func (h *Client) InstallRelease(chstr, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	// load the chart to install
	chart, err := chartutil.Load(chstr)
	if err != nil {
		return nil, err
	}

	return h.InstallReleaseFromChart(chart, ns, opts...)
}

// InstallReleaseFromChart installs a new chart and returns the release response.
func (h *Client) InstallReleaseFromChart(chart *hapi_chart.Chart, ns string, opts ...InstallOption) (*rls.InstallReleaseResponse, error) {
	// apply the install options
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.instReq
	req.Chart = chart
	req.Namespace = ns
	req.DryRun = h.opts.dryRun
	req.DisableHooks = h.opts.disableHooks
	req.ReuseName = h.opts.reuseName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.install(ctx, req)
}

// DeleteRelease uninstalls a named release and returns the response.
func (h *Client) DeleteRelease(rlsName string, namespace string, opts ...DeleteOption) (*rls.UninstallReleaseResponse, error) {
	// apply the uninstall options
	for _, opt := range opts {
		opt(&h.opts)
	}

	if h.opts.dryRun {
		// In the dry run case, just see if the release exists
		r, err := h.ReleaseContent(rlsName)
		if err != nil {
			return &rls.UninstallReleaseResponse{}, err
		}
		return &rls.UninstallReleaseResponse{Release: r.Release}, nil
	}

	req := &h.opts.uninstallReq
	req.Name = rlsName
	req.DisableHooks = h.opts.disableHooks
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.delete(ctx, namespace, req)
}

// UpdateRelease updates a release to a new/different chart
func (h *Client) UpdateRelease(rlsName string, chstr string, namespace string, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {
	// load the chart to update
	chart, err := chartutil.Load(chstr)
	if err != nil {
		return nil, err
	}

	return h.UpdateReleaseFromChart(rlsName, chart, namespace, opts...)
}

// UpdateReleaseFromChart updates a release to a new/different chart
func (h *Client) UpdateReleaseFromChart(rlsName string, chart *hapi_chart.Chart, namespace string, opts ...UpdateOption) (*rls.UpdateReleaseResponse, error) {

	// apply the update options
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.updateReq
	req.Chart = chart
	req.DryRun = h.opts.dryRun
	req.Name = rlsName
	req.DisableHooks = h.opts.disableHooks
	req.Recreate = h.opts.recreate
	req.ResetValues = h.opts.resetValues
	ctx := NewContext()
	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.update(ctx, namespace, req)
}

// GetVersion returns the server version
func (h *Client) GetVersion(opts ...VersionOption) (*rls.GetVersionResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &rls.GetVersionRequest{}
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.version(ctx, req)
}

// RollbackRelease rolls back a release to the previous version
func (h *Client) RollbackRelease(rlsName string, namespace string, opts ...RollbackOption) (*rls.RollbackReleaseResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.rollbackReq
	req.DisableHooks = h.opts.disableHooks
	req.DryRun = h.opts.dryRun
	req.Name = rlsName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.rollback(ctx, namespace, req)
}

// ReleaseStatus returns the given release's status.
func (h *Client) ReleaseStatus(rlsName string, namespace string, opts ...StatusOption) (*rls.GetReleaseStatusResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.statusReq
	req.Name = rlsName
	req.Version = h.opts.statusReq.Version
	ctx := NewContext()
	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.status(ctx, namespace, req)
}

// ReleaseContent returns the configuration for a given release.
func (h *Client) ReleaseContent(rlsName string, opts ...ContentOption) (*rls.GetReleaseContentResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}
	req := &h.opts.contentReq
	req.Name = rlsName
	ctx := NewContext()
	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.content(ctx, req)
}

// ReleaseHistory returns a release's revision history.
func (h *Client) ReleaseHistory(rlsName string, opts ...HistoryOption) (*rls.GetHistoryResponse, error) {
	for _, opt := range opts {
		opt(&h.opts)
	}

	req := &h.opts.histReq
	req.Name = rlsName
	ctx := NewContext()

	if h.opts.before != nil {
		if err := h.opts.before(ctx, req); err != nil {
			return nil, err
		}
	}
	return h.history(ctx, req)
}

// Executes tiller.ListReleases RPC.
func (h *Client) list(ctx context.Context, req *rls.ListReleasesRequest) (*rls.ListReleasesResponse, error) {
	c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	s, err := rlc.ListReleases(ctx, req)
	if err != nil {
		return nil, err
	}

	return s.Recv()
}

// Executes tiller.InstallRelease RPC.
func (h *Client) install(ctx context.Context, req *rls.InstallReleaseRequest) (*rls.InstallReleaseResponse, error) {
	/*			c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
				if err != nil {
					return nil, err
				}
				defer c.Close()
				rlc := rls.NewReleaseServiceClient(c)
				return rlc.InstallRelease(ctx, req)*/
	resp := &rls.InstallReleaseResponse{}
	maxTries := 5
	releaseNameMaxLen := 53
	name := req.Name
	client, err := getRESTClient()
	if err != nil {
		return resp, err
	}
	// make uniq name
	if len(name) == 0 {
		for i := 0; i < maxTries; i++ {
			namer := moniker.New()
			name = namer.NameSep("-")
			if len(name) > releaseNameMaxLen {
				name = name[:releaseNameMaxLen]
			}
			rv := name + "-v1"
			err = client.RESTClient().Get().Namespace(req.Namespace).Resource("releaseversion").Name(rv).Do().Error()
			if err == nil {
				fmt.Println("info: Name %q is taken. Searching again.", name)
			} else {
				break
			}
		}
	}
	req.Name = name
	releaseObj := makeReleaseObject(req)
	releaseObj.Spec.Version = 1
	release := new(hapi.Release)
	err = client.RESTClient().Post().Namespace(req.Namespace).Resource("releases").Body(releaseObj).Do().Into(release)
	if err != nil {
		return resp, err
	}
	resp.Release = new(rs.Release)
	resp.Release.Name = release.Name
	resp.Release.Namespace = release.Namespace
	resp.Release.Hooks = release.Spec.Hooks
	resp.Release.Config = release.Spec.Config
	resp.Release.Chart = new(hapi_chart.Chart)
	resp.Release.Chart = release.Spec.Chart.Inline
	resp.Release.Manifest = release.Spec.Manifest
	resp.Release.Info = new(rs.Info)
	resp.Release.Info.Status = new(rs.Status)
	resp.Release.Info.Status.Notes = release.Status.Notes
	resp.Release.Info.Status.Code = release.Status.Code
	resp.Release.Info.Status.Details = release.Status.Details
	firstDeployed, err := ptypes.TimestampProto(release.Status.FirstDeployed.Time)
	if err != nil {
		return resp, err
	}
	resp.Release.Info.FirstDeployed = firstDeployed
	lastDeployed, err := ptypes.TimestampProto(release.Status.LastDeployed.Time)
	if err != nil {
		return resp, err
	}
	resp.Release.Info.FirstDeployed = lastDeployed
	return resp, nil
}

// Executes tiller.UninstallRelease RPC.
func (h *Client) delete(ctx context.Context, namespace string, req *rls.UninstallReleaseRequest) (*rls.UninstallReleaseResponse, error) {
	/*	c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		defer c.Close()

		rlc := rls.NewReleaseServiceClient(c)
		return rlc.UninstallRelease(ctx, req)*/
	resp := &rls.UninstallReleaseResponse{}
	client, err := getRESTClient()
	// TODO handle response
	//release := new(hapi.Release)
	err = client.RESTClient().Delete().Namespace(namespace).Resource("releases").Name(req.Name).Do().Error()
	if err != nil {
		return resp, err
	}
	return resp, nil
}

// Executes tiller.UpdateRelease RPC.
func (h *Client) update(ctx context.Context, namespace string, req *rls.UpdateReleaseRequest) (*rls.UpdateReleaseResponse, error) {
	/*	c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		defer c.Close()

		rlc := rls.NewReleaseServiceClient(c)
		return rlc.UpdateRelease(ctx, req)*/
	resp := &rls.UpdateReleaseResponse{}
	client, err := getRESTClient()
	// get the release
	release := new(hapi.Release)
	err = client.RESTClient().Get().Namespace(namespace).Resource("releases").Name(req.Name).Do().Into(release)
	if err != nil {
		return resp, err
	}
	release.Spec.Config = req.Values
	release.Spec.DryRun = req.DryRun
	release.Spec.DisableHooks = req.DisableHooks
	release.Spec.Recreate = req.Recreate
	release.Spec.Timeout = req.Timeout
	release.Spec.Chart.Inline = req.Chart
	release.Spec.Version = release.Spec.Version + 1
	// update the release
	updatedRelease := new(hapi.Release)
	err = client.RESTClient().Put().Namespace(namespace).Resource("releases").Name(release.Name).Body(release).Do().Into(updatedRelease)
	if err != nil {
		return resp, err
	}
	resp.Release = new(rs.Release)
	resp.Release.Name = updatedRelease.Name
	resp.Release.Chart = updatedRelease.Spec.Chart.Inline
	resp.Release.Config = updatedRelease.Spec.Config
	resp.Release.Manifest = updatedRelease.Spec.Manifest
	resp.Release.Hooks = updatedRelease.Spec.Hooks
	resp.Release.Version = updatedRelease.Spec.Version
	resp.Release.Info = new(rs.Info)
	resp.Release.Info.Status = new(rs.Status)
	resp.Release.Info.Status.Code = updatedRelease.Status.Code
	resp.Release.Info.Status.Details = updatedRelease.Status.Details
	resp.Release.Info.Status.Notes = updatedRelease.Status.Notes
	return resp, nil
}

// Executes tiller.RollbackRelease RPC.
func (h *Client) rollback(ctx context.Context, namespace string, req *rls.RollbackReleaseRequest) (*rls.RollbackReleaseResponse, error) {
	/*	c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		defer c.Close()

		rlc := rls.NewReleaseServiceClient(c)
		return rlc.RollbackRelease(ctx, req)*/
	resp := &rls.RollbackReleaseResponse{}
	config, err := getConfig()
	if err != nil {
		return resp, err
	}
	extClient, err := getRESTClient()
	client, err := clientset.NewForConfig(config)
	err = extClient.RESTClient().Get().Namespace(namespace).Resource("releases").Name(req.Name).Do().Error()
	if err != nil {
		return resp, errors.New("Release not found")
	}
	event, err := makeEventForRollBack(req)
	event.ObjectMeta.Namespace = namespace
	event.InvolvedObject.Namespace = namespace
	event.InvolvedObject.Name = (req.Name + "-v" + strconv.Itoa(int(req.Version)))
	event.ObjectMeta.Name = event.InvolvedObject.Name + "-" + RandStringRunes(5)
	if err != nil {
		return resp, err
	}
	err = extClient.RESTClient().Get().Namespace(namespace).Resource("releaseversions").Name(event.InvolvedObject.Name).Do().Error()
	if err != nil {
		return resp, errors.New("Release Version not found")
	}
	_, err = client.Core().Events(namespace).Create(event)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

// Executes tiller.GetReleaseStatus RPC.
func (h *Client) status(ctx context.Context, namespace string, req *rls.GetReleaseStatusRequest) (*rls.GetReleaseStatusResponse, error) {
	/*
		c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		defer c.Close()

		rlc := rls.NewReleaseServiceClient(c)
		return rlc.GetReleaseStatus(ctx, req)

	*/
	resp := &rls.GetReleaseStatusResponse{}
	client, err := getRESTClient()
	if err != nil {
		return resp, err
	}
	release := new(hapi.Release)
	resp.Info = new(rs.Info)
	resp.Info.Status = new(rs.Status)
	if req.Version == 0 {
		err = client.RESTClient().Get().Namespace(namespace).Resource("releases").Name(req.Name).Do().Into(release)
		if err != nil {
			e := client.RESTClient().Get().Namespace(namespace).Resource("releaseversions").Name(req.Name + "-v1").Do().Error()
			if e != nil {
				return resp, err
			} else {
				resp.Namespace = namespace
				resp.Info.Status.Code = rs.Status_DELETED
				return resp, nil
			}
		}
		req.Version = release.Spec.Version
	}
	duration := time.Duration(2) * time.Second
	releaseVersion := new(hapi.ReleaseVersion)
	name := req.Name + "-v" + strconv.Itoa(int(req.Version))
	for i := 0; i <= 20; i++ {
		err = client.RESTClient().Get().Namespace(namespace).Resource("releaseversions").Name(name).Do().Into(releaseVersion)
		if err != nil {
			time.Sleep(duration)
			continue
		} else {
			break
		}
	}
	if err != nil {
		return resp, err
	}
	err = client.RESTClient().Get().Namespace(namespace).Resource("releases").Name(req.Name).Do().Into(release)
	if err != nil {
		return resp, err
	}
	resp.Name = release.Name
	resp.Namespace = release.Namespace

	resp.Info.Status.Code = release.Status.Code
	resp.Info.Status.Notes = release.Status.Notes
	resp.Info.Status.Details = release.Status.Details
	resp.Info.Status.Resources = GetLatestResourceStatus(release.Namespace, release.Spec.Manifest)
	f, err := ptypes.TimestampProto(release.Status.FirstDeployed.Time)
	if err != nil {
		return resp, err
	}
	resp.Info.FirstDeployed = f
	l, err := ptypes.TimestampProto(release.Status.LastDeployed.Time)
	if err != nil {
		return resp, err
	}
	resp.Info.LastDeployed = l

	return resp, nil
}

// Executes tiller.GetReleaseContent RPC.
func (h *Client) content(ctx context.Context, req *rls.GetReleaseContentRequest) (*rls.GetReleaseContentResponse, error) {
	c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.GetReleaseContent(ctx, req)
}

// Executes tiller.GetVersion RPC.
func (h *Client) version(ctx context.Context, req *rls.GetVersionRequest) (*rls.GetVersionResponse, error) {
	c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.GetVersion(ctx, req)
}

// Executes tiller.GetHistory RPC.
func (h *Client) history(ctx context.Context, req *rls.GetHistoryRequest) (*rls.GetHistoryResponse, error) {
	c, err := grpc.Dial(h.opts.host, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer c.Close()

	rlc := rls.NewReleaseServiceClient(c)
	return rlc.GetHistory(ctx, req)
}

func makeReleaseObject(req *rls.InstallReleaseRequest) *hapi.Release {
	release := &hapi.Release{}
	release.TypeMeta.Kind = "Release"
	release.TypeMeta.APIVersion = "helm.sh/v1beta1"
	release.ObjectMeta.Name = req.Name
	release.ObjectMeta.Namespace = req.Namespace
	release.Spec = makeObjectSpec(req)
	return release
}
func makeObjectSpec(req *rls.InstallReleaseRequest) hapi.ReleaseSpec {
	spec := hapi.ReleaseSpec{}
	spec.DryRun = req.DryRun
	spec.DisableHooks = req.DisableHooks
	// spec.Reuse = req.ReuseName TODO To enable reuse in installation
	spec.Config = req.Values
	spec.Chart.Inline = new(hapi_chart.Chart)
	spec.Chart.Inline.Files = req.Chart.Files
	spec.Chart.Inline.Metadata = req.Chart.Metadata
	spec.Chart.Inline.Templates = req.Chart.Templates
	spec.Chart.Inline.Values = req.Chart.Values
	return spec
}

func getConfig() (*restclient.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("Could not get kubernetes config: %s", err)
	}
	return config, nil
}

func getRESTClient() (*cs.ExtensionsClient, error) {
	c, err := getConfig()
	config := *c
	if err != nil {
		return nil, err
	}
	client, err := cs.NewExtensionsForConfig(&config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type RollbackReq struct {
	// dry_run, if true, will run through the release logic but no create
	DryRun bool `protobuf:"varint,2,opt,name=dry_run,json=dryRun" json:"dry_run,omitempty"`
	// DisableHooks causes the server to skip running any hooks for the rollback
	DisableHooks bool `protobuf:"varint,3,opt,name=disable_hooks,json=disableHooks" json:"disable_hooks,omitempty"`
	// Performs pods restart for resources if applicable
	Recreate bool `protobuf:"varint,5,opt,name=recreate" json:"recreate,omitempty"`
	// timeout specifies the max amount of time any kubernetes client command can run.
	Timeout int64 `protobuf:"varint,6,opt,name=timeout" json:"timeout,omitempty"`
}

func makeEventForRollBack(req *rls.RollbackReleaseRequest) (*api.Event, error) {
	r := RollbackReq{
		DryRun:       req.DryRun,
		Recreate:     req.Recreate,
		DisableHooks: req.DisableHooks,
		Timeout:      req.Timeout,
	}
	message, err := json.Marshal(r)
	if err != nil {
		return &api.Event{}, err
	}
	event := &api.Event{
		InvolvedObject: api.ObjectReference{
			Kind: "release",
		},
		Reason:  "releaseRollback",
		Message: string(message),
		Type:    api.EventTypeNormal,
		Count:   1,
	}
	return event, nil
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func GetLatestResourceStatus(namespace, manifest string) string {
	KubeClient := kube.New(nil)
	resource, err := KubeClient.Get(namespace, bytes.NewBufferString(manifest))
	if err != nil {
		log.Println(err)
		return ""
	}
	return resource
}
