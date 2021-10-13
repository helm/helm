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

package registry // import "helm.sh/helm/v3/internal/experimental/registry"

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/containerd/containerd/remotes"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"oras.land/oras-go/pkg/auth"
	dockerauth "oras.land/oras-go/pkg/auth/docker"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"

	"helm.sh/helm/v3/internal/version"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/helmpath"
)

type (
	// Client works with OCI-compliant registries
	Client struct {
		debug bool
		// path to repository config file e.g. ~/.docker/config.json
		credentialsFile string
		out             io.Writer
		authorizer      auth.Client
		resolver        remotes.Resolver
	}

	// ClientOption allows specifying various settings configurable by the user for overriding the defaults
	// used when creating a new default client
	ClientOption func(*Client)
)

// NewClient returns a new registry client with config
func NewClient(options ...ClientOption) (*Client, error) {
	client := &Client{
		out: ioutil.Discard,
	}
	for _, option := range options {
		option(client)
	}
	if client.credentialsFile == "" {
		client.credentialsFile = helmpath.ConfigPath(CredentialsFileBasename)
	}
	if client.authorizer == nil {
		authClient, err := dockerauth.NewClient(client.credentialsFile)
		if err != nil {
			return nil, err
		}
		client.authorizer = authClient
	}
	if client.resolver == nil {
		headers := http.Header{}
		headers.Set("User-Agent", version.GetUserAgent())
		opts := []auth.ResolverOption{auth.WithResolverHeaders(headers)}
		resolver, err := client.authorizer.ResolverWithOpts(opts...)
		if err != nil {
			return nil, err
		}
		client.resolver = resolver
	}
	return client, nil
}

// ClientOptDebug returns a function that sets the debug setting on client options set
func ClientOptDebug(debug bool) ClientOption {
	return func(client *Client) {
		client.debug = debug
	}
}

// ClientOptWriter returns a function that sets the writer setting on client options set
func ClientOptWriter(out io.Writer) ClientOption {
	return func(client *Client) {
		client.out = out
	}
}

// ClientOptCredentialsFile returns a function that sets the credentialsFile setting on a client options set
func ClientOptCredentialsFile(credentialsFile string) ClientOption {
	return func(client *Client) {
		client.credentialsFile = credentialsFile
	}
}

type (
	// LoginOption allows specifying various settings on login
	LoginOption func(*loginOperation)

	loginOperation struct {
		username string
		password string
		insecure bool
	}
)

// Login logs into a registry
func (c *Client) Login(host string, options ...LoginOption) error {
	operation := &loginOperation{}
	for _, option := range options {
		option(operation)
	}
	authorizerLoginOpts := []auth.LoginOption{
		auth.WithLoginContext(ctx(c.out, c.debug)),
		auth.WithLoginHostname(host),
		auth.WithLoginUsername(operation.username),
		auth.WithLoginSecret(operation.password),
		auth.WithLoginUserAgent(version.GetUserAgent()),
	}
	if operation.insecure {
		authorizerLoginOpts = append(authorizerLoginOpts, auth.WithLoginInsecure())
	}
	if err := c.authorizer.LoginWithOpts(authorizerLoginOpts...); err != nil {
		return err
	}
	fmt.Fprintln(c.out, "Login Succeeded")
	return nil
}

// LoginOptBasicAuth returns a function that sets the username/password settings on login
func LoginOptBasicAuth(username string, password string) LoginOption {
	return func(operation *loginOperation) {
		operation.username = username
		operation.password = password
	}
}

// LoginOptInsecure returns a function that sets the insecure setting on login
func LoginOptInsecure(insecure bool) LoginOption {
	return func(operation *loginOperation) {
		operation.insecure = insecure
	}
}

type (
	// LogoutOption allows specifying various settings on logout
	LogoutOption func(*logoutOperation)

	logoutOperation struct{}
)

// Logout logs out of a registry
func (c *Client) Logout(host string, opts ...LogoutOption) error {
	operation := &logoutOperation{}
	for _, opt := range opts {
		opt(operation)
	}
	if err := c.authorizer.Logout(ctx(c.out, c.debug), host); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "Removing login credentials for %s\n", host)
	return nil
}

type (
	// PullOption allows specifying various settings on pull
	PullOption func(*pullOperation)

	// PullResult is the result returned upon successful pull.
	PullResult struct {
		Manifest *descriptorPullSummary         `json:"manifest"`
		Config   *descriptorPullSummary         `json:"config"`
		Chart    *descriptorPullSummaryWithMeta `json:"chart"`
		Prov     *descriptorPullSummary         `json:"prov"`
		Ref      string                         `json:"ref"`
	}

	descriptorPullSummary struct {
		Data   []byte `json:"-"`
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	}

	descriptorPullSummaryWithMeta struct {
		descriptorPullSummary
		Meta *chart.Metadata `json:"meta"`
	}

	pullOperation struct {
		withChart         bool
		withProv          bool
		ignoreMissingProv bool
	}
)

// Pull downloads a chart from a registry
func (c *Client) Pull(ref string, options ...PullOption) (*PullResult, error) {
	operation := &pullOperation{
		withChart: true, // By default, always download the chart layer
	}
	for _, option := range options {
		option(operation)
	}
	if !operation.withChart && !operation.withProv {
		return nil, errors.New(
			"must specify at least one layer to pull (chart/prov)")
	}
	store := content.NewMemoryStore()
	allowedMediaTypes := []string{
		ConfigMediaType,
	}
	minNumDescriptors := 1 // 1 for the config
	if operation.withChart {
		minNumDescriptors++
		allowedMediaTypes = append(allowedMediaTypes, ChartLayerMediaType, LegacyChartLayerMediaType)
	}
	if operation.withProv {
		if !operation.ignoreMissingProv {
			minNumDescriptors++
		}
		allowedMediaTypes = append(allowedMediaTypes, ProvLayerMediaType)
	}
	manifest, descriptors, err := oras.Pull(ctx(c.out, c.debug), c.resolver, ref, store,
		oras.WithPullEmptyNameAllowed(),
		oras.WithAllowedMediaTypes(allowedMediaTypes))
	if err != nil {
		return nil, err
	}
	numDescriptors := len(descriptors)
	if numDescriptors < minNumDescriptors {
		return nil, errors.New(
			fmt.Sprintf("manifest does not contain minimum number of descriptors (%d), descriptors found: %d",
				minNumDescriptors, numDescriptors))
	}
	var configDescriptor *ocispec.Descriptor
	var chartDescriptor *ocispec.Descriptor
	var provDescriptor *ocispec.Descriptor
	for _, descriptor := range descriptors {
		d := descriptor
		switch d.MediaType {
		case ConfigMediaType:
			configDescriptor = &d
		case ChartLayerMediaType:
			chartDescriptor = &d
		case ProvLayerMediaType:
			provDescriptor = &d
		case LegacyChartLayerMediaType:
			chartDescriptor = &d
			fmt.Fprintf(c.out, "Warning: chart media type %s is deprecated\n", LegacyChartLayerMediaType)
		}
	}
	if configDescriptor == nil {
		return nil, errors.New(
			fmt.Sprintf("could not load config with mediatype %s", ConfigMediaType))
	}
	if operation.withChart && chartDescriptor == nil {
		return nil, errors.New(
			fmt.Sprintf("manifest does not contain a layer with mediatype %s",
				ChartLayerMediaType))
	}
	var provMissing bool
	if operation.withProv && provDescriptor == nil {
		if operation.ignoreMissingProv {
			provMissing = true
		} else {
			return nil, errors.New(
				fmt.Sprintf("manifest does not contain a layer with mediatype %s",
					ProvLayerMediaType))
		}
	}
	result := &PullResult{
		Manifest: &descriptorPullSummary{
			Digest: manifest.Digest.String(),
			Size:   manifest.Size,
		},
		Config: &descriptorPullSummary{
			Digest: configDescriptor.Digest.String(),
			Size:   configDescriptor.Size,
		},
		Chart: &descriptorPullSummaryWithMeta{},
		Prov:  &descriptorPullSummary{},
		Ref:   ref,
	}
	var getManifestErr error
	if _, manifestData, ok := store.Get(manifest); !ok {
		getManifestErr = errors.Errorf("Unable to retrieve blob with digest %s", manifest.Digest)
	} else {
		result.Manifest.Data = manifestData
	}
	if getManifestErr != nil {
		return nil, getManifestErr
	}
	var getConfigDescriptorErr error
	if _, configData, ok := store.Get(*configDescriptor); !ok {
		getConfigDescriptorErr = errors.Errorf("Unable to retrieve blob with digest %s", configDescriptor.Digest)
	} else {
		result.Config.Data = configData
		var meta *chart.Metadata
		if err := json.Unmarshal(configData, &meta); err != nil {
			return nil, err
		}
		result.Chart.Meta = meta
	}
	if getConfigDescriptorErr != nil {
		return nil, getConfigDescriptorErr
	}
	if operation.withChart {
		var getChartDescriptorErr error
		if _, chartData, ok := store.Get(*chartDescriptor); !ok {
			getChartDescriptorErr = errors.Errorf("Unable to retrieve blob with digest %s", chartDescriptor.Digest)
		} else {
			result.Chart.Data = chartData
			result.Chart.Digest = chartDescriptor.Digest.String()
			result.Chart.Size = chartDescriptor.Size
		}
		if getChartDescriptorErr != nil {
			return nil, getChartDescriptorErr
		}
	}
	if operation.withProv && !provMissing {
		var getProvDescriptorErr error
		if _, provData, ok := store.Get(*provDescriptor); !ok {
			getProvDescriptorErr = errors.Errorf("Unable to retrieve blob with digest %s", provDescriptor.Digest)
		} else {
			result.Prov.Data = provData
			result.Prov.Digest = provDescriptor.Digest.String()
			result.Prov.Size = provDescriptor.Size
		}
		if getProvDescriptorErr != nil {
			return nil, getProvDescriptorErr
		}
	}
	fmt.Fprintf(c.out, "Pulled: %s\n", result.Ref)
	fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)
	return result, nil
}

// PullOptWithChart returns a function that sets the withChart setting on pull
func PullOptWithChart(withChart bool) PullOption {
	return func(operation *pullOperation) {
		operation.withChart = withChart
	}
}

// PullOptWithProv returns a function that sets the withProv setting on pull
func PullOptWithProv(withProv bool) PullOption {
	return func(operation *pullOperation) {
		operation.withProv = withProv
	}
}

// PullOptIgnoreMissingProv returns a function that sets the ignoreMissingProv setting on pull
func PullOptIgnoreMissingProv(ignoreMissingProv bool) PullOption {
	return func(operation *pullOperation) {
		operation.ignoreMissingProv = ignoreMissingProv
	}
}

type (
	// PushOption allows specifying various settings on push
	PushOption func(*pushOperation)

	// PushResult is the result returned upon successful push.
	PushResult struct {
		Manifest *descriptorPushSummary         `json:"manifest"`
		Config   *descriptorPushSummary         `json:"config"`
		Chart    *descriptorPushSummaryWithMeta `json:"chart"`
		Prov     *descriptorPushSummary         `json:"prov"`
		Ref      string                         `json:"ref"`
	}

	descriptorPushSummary struct {
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	}

	descriptorPushSummaryWithMeta struct {
		descriptorPushSummary
		Meta *chart.Metadata `json:"meta"`
	}

	pushOperation struct {
		provData   []byte
		strictMode bool
	}
)

// Push uploads a chart to a registry.
func (c *Client) Push(data []byte, ref string, options ...PushOption) (*PushResult, error) {
	operation := &pushOperation{
		strictMode: true, // By default, enable strict mode
	}
	for _, option := range options {
		option(operation)
	}
	meta, err := extractChartMeta(data)
	if err != nil {
		return nil, err
	}
	if operation.strictMode {
		if !strings.HasSuffix(ref, fmt.Sprintf("/%s:%s", meta.Name, meta.Version)) {
			return nil, errors.New(
				"strict mode enabled, ref basename and tag must match the chart name and version")
		}
	}
	store := content.NewMemoryStore()
	chartDescriptor := store.Add("", ChartLayerMediaType, data)
	configData, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	configDescriptor := store.Add("", ConfigMediaType, configData)
	descriptors := []ocispec.Descriptor{chartDescriptor}
	var provDescriptor ocispec.Descriptor
	if operation.provData != nil {
		provDescriptor = store.Add("", ProvLayerMediaType, operation.provData)
		descriptors = append(descriptors, provDescriptor)
	}
	manifest, err := oras.Push(ctx(c.out, c.debug), c.resolver, ref, store, descriptors,
		oras.WithConfig(configDescriptor), oras.WithNameValidation(nil))
	if err != nil {
		return nil, err
	}
	chartSummary := &descriptorPushSummaryWithMeta{
		Meta: meta,
	}
	chartSummary.Digest = chartDescriptor.Digest.String()
	chartSummary.Size = chartDescriptor.Size
	result := &PushResult{
		Manifest: &descriptorPushSummary{
			Digest: manifest.Digest.String(),
			Size:   manifest.Size,
		},
		Config: &descriptorPushSummary{
			Digest: configDescriptor.Digest.String(),
			Size:   configDescriptor.Size,
		},
		Chart: chartSummary,
		Prov:  &descriptorPushSummary{}, // prevent nil references
		Ref:   ref,
	}
	if operation.provData != nil {
		result.Prov = &descriptorPushSummary{
			Digest: provDescriptor.Digest.String(),
			Size:   provDescriptor.Size,
		}
	}
	fmt.Fprintf(c.out, "Pushed: %s\n", result.Ref)
	fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)
	return result, err
}

// PushOptProvData returns a function that sets the prov bytes setting on push
func PushOptProvData(provData []byte) PushOption {
	return func(operation *pushOperation) {
		operation.provData = provData
	}
}

// PushOptStrictMode returns a function that sets the strictMode setting on push
func PushOptStrictMode(strictMode bool) PushOption {
	return func(operation *pushOperation) {
		operation.strictMode = strictMode
	}
}
