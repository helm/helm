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

package registry // import "helm.sh/helm/v4/pkg/registry"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3"
	"github.com/oras-project/oras-go/v3/content/memory"
	"github.com/oras-project/oras-go/v3/registry/remote"
	"github.com/oras-project/oras-go/v3/registry/remote/auth"
	remoteconfig "github.com/oras-project/oras-go/v3/registry/remote/config"
	"github.com/oras-project/oras-go/v3/registry/remote/credentials"
	"github.com/oras-project/oras-go/v3/registry/remote/policy"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
	"github.com/oras-project/oras-go/v3/registry/remote/retry"
	"github.com/oras-project/oras-go/v3/registry/remote/signature"

	"helm.sh/helm/v4/internal/version"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/helmpath"
)

// See https://github.com/helm/helm/issues/10166
const registryUnderscoreMessage = `
OCI artifact references (e.g. tags) do not support the plus sign (+). To support
storing semantic versions, Helm adopts the convention of changing plus (+) to
an underscore (_) in chart version tags when pushing to a registry and back to
a plus (+) when pulling from a registry.`

// ConfigOptions specifies override paths for container ecosystem config files.
type ConfigOptions struct {
	RegistriesConfigPath string
	PolicyConfigPath     string
	CertsDirPaths        []string
	ContainersAuthPath   string
}

type (
	// RemoteClient shadows the ORAS remote.Client interface
	// (hiding the ORAS type from Helm client visibility)
	// https://pkg.go.dev/oras.land/oras-go/pkg/registry/remote#Client
	RemoteClient interface {
		Do(req *http.Request) (*http.Response, error)
	}

	// Client works with OCI-compliant registries
	Client struct {
		debug       bool
		enableCache bool
		// path to repository config file e.g. ~/.docker/config.json
		credentialsFile    string
		username           string
		password           string
		out                io.Writer
		authorizer         *auth.Client
		registryAuthorizer RemoteClient
		credentialsStore   credentials.Store
		httpClient         *http.Client
		plainHTTP          bool
		// v3 config-driven fields
		configs               *remoteconfig.Configs
		builder               *remote.ClientBuilder
		policyEvaluator       *policy.Evaluator
		signatureVerification bool
		configOptions         ConfigOptions
		insecure              bool
		certFile              string
		keyFile               string
		caFile                string
		customHTTPClient      bool // true when ClientOptHTTPClient or ClientOptAuthorizer was used
	}

	// ClientOption allows specifying various settings configurable by the user for overriding the defaults
	// used when creating a new default client
	// TODO(TerryHowe): ClientOption should return error in v5
	ClientOption func(*Client)
)

// NewClient returns a new registry client with config
func NewClient(options ...ClientOption) (*Client, error) {
	client := &Client{
		out: io.Discard,
	}
	for _, option := range options {
		option(client)
	}
	if client.credentialsFile == "" {
		client.credentialsFile = helmpath.ConfigPath(CredentialsFileBasename)
	}
	if client.httpClient == nil {
		client.httpClient = &http.Client{
			Transport: NewTransport(client.debug),
		}
	}

	storeOptions := credentials.StoreOptions{
		AllowPlaintextPut: true,
	}

	// Primary credentials store (Helm's own file) — used for Login/Logout persistence.
	helmStore, err := credentials.NewStore(client.credentialsFile, storeOptions)
	if err != nil {
		return nil, err
	}
	client.credentialsStore = helmStore

	// Load the full container ecosystem config stack: Docker config.json,
	// containers auth.json, registries.conf, policy.json, certs.d, registries.d.
	// Missing files are silently skipped.
	loaderOpts := remoteconfig.LoadConfigsOptions{
		RegistriesConfigPath: client.configOptions.RegistriesConfigPath,
		PolicyConfigPath:     client.configOptions.PolicyConfigPath,
		CertsDirPaths:        client.configOptions.CertsDirPaths,
		ContainersAuthPath:   client.configOptions.ContainersAuthPath,
	}
	configs, err := remoteconfig.LoadConfigsWithOptions(loaderOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to load registry configurations: %w", err)
	}
	client.configs = configs

	// Only build from configs when not overridden by ClientOptPolicyEvaluator.
	if configs.PolicyConfig != nil && client.policyEvaluator == nil {
		evaluator, err := configs.PolicyEvaluator()
		if err != nil {
			return nil, fmt.Errorf("failed to build policy evaluator: %w", err)
		}
		client.policyEvaluator = evaluator
	}

	// Build the combined credential store for read operations:
	//   1. Helm's own credentials file (highest priority, stores login results)
	//   2. Docker config.json + containers auth.json from the full config stack
	var credStore credentials.Store
	if configStore, err := configs.CredentialStore(storeOptions); err == nil {
		credStore = credentials.NewStoreWithFallbacks(helmStore, configStore)
	} else {
		credStore = helmStore
	}

	// Build the ClientBuilder used by the config-driven repository creation path.
	var logger *slog.Logger
	if client.debug {
		logger = slog.Default()
	}
	builder := remote.NewClientBuilder()
	builder.CredentialStore = credStore
	builder.UserAgent = version.GetUserAgent()
	builder.Logger = logger
	builder.PolicyEvaluator = client.policyEvaluator
	if !client.enableCache {
		builder.CacheFactory = nil
	}
	client.builder = builder

	// Build the legacy auth.Client used when a custom HTTP client or authorizer
	// was provided (customHTTPClient=true path).
	if client.authorizer == nil {
		authorizer := auth.Client{
			Client: client.httpClient,
		}
		authorizer.SetUserAgent(version.GetUserAgent())

		if client.username != "" && client.password != "" {
			authorizer.CredentialFunc = func(_ context.Context, _ string) (credentials.Credential, error) {
				return credentials.Credential{Username: client.username, Password: client.password}, nil
			}
		} else {
			authorizer.CredentialFunc = remote.NewCredentialFunc(credStore)
		}

		if client.enableCache {
			authorizer.Cache = auth.NewCache()
		}
		client.authorizer = &authorizer
	}

	return client, nil
}

// applyOverrides applies client-level CLI flag overrides to registry properties.
func (c *Client) applyOverrides(props *properties.Registry) {
	if c.plainHTTP {
		props.Transport.PlainHTTP = true
	}
	if c.insecure {
		props.Transport.Insecure = true
	}
	if c.certFile != "" && c.keyFile != "" {
		props.Transport.Cert = c.certFile
		props.Transport.Key = c.keyFile
	}
	if c.caFile != "" {
		props.Transport.CACerts = append(props.Transport.CACerts, c.caFile)
	}
	if c.username != "" {
		props.Credential = credentials.Credential{Username: c.username, Password: c.password}
	}
}

// newRepository creates a configured remote.Repository for the given reference.
//
// When a custom HTTP client or authorizer was provided, the legacy path is used
// (direct assignment of c.authorizer). Otherwise, the full config-driven path is
// used: registry properties are resolved from registries.conf and certs.d, CLI
// overrides are applied, and the repository is built via NewRepositoryWithProperties.
func (c *Client) newRepository(ref string) (*remote.Repository, error) {
	if c.customHTTPClient {
		// Legacy path: use c.authorizer directly (preserves custom TLS transport).
		repo, err := remote.NewRepository(ref)
		if err != nil {
			return nil, err
		}
		repo.Registry.PlainHTTP = c.plainHTTP
		repo.Registry.Client = c.authorizer
		repo.Registry.Policy = c.policyEvaluator
		return repo, nil
	}
	// Config-driven path: resolve properties from registries.conf, certs.d, etc.
	props, err := c.configs.RegistryProperties(ref)
	if err != nil {
		return nil, err
	}
	c.applyOverrides(props)
	repo, err := remote.NewRepositoryWithProperties(props, c.builder)
	if err != nil {
		return nil, err
	}
	if c.signatureVerification && c.configs.RegistriesDConfig != nil && c.policyEvaluator != nil {
		scope := props.Reference.Registry
		if props.Reference.Repository != "" {
			scope += "/" + props.Reference.Repository
		}
		verifier := signature.NewSignedByVerifierFromConfig(c.configs.RegistriesDConfig, scope)
		if verifier != nil {
			scopedEval, err := c.configs.PolicyEvaluator(policy.WithSignedByVerifier(verifier))
			if err == nil && scopedEval != nil {
				repo.Registry.Policy = scopedEval
			}
		}
	}
	return repo, nil
}

// newRegistry creates a configured remote.Registry for the given host (used by Login).
func (c *Client) newRegistry(host string) (*remote.Registry, error) {
	if c.customHTTPClient {
		// Legacy path: use c.authorizer directly.
		reg, err := remote.NewRegistry(host)
		if err != nil {
			return nil, err
		}
		reg.PlainHTTP = c.plainHTTP
		reg.Client = c.authorizer
		return reg, nil
	}
	// Config-driven path: construct properties for a host-only reference
	// (no repository path) and apply CLI overrides + certs.d.
	props := properties.NewRegistryFromReference(properties.Reference{Registry: host})
	// Apply insecure setting from registries.conf if the host is found there.
	if c.configs.RegistriesConfig != nil {
		if reg := c.configs.RegistriesConfig.FindRegistry(host); reg != nil && reg.Insecure {
			props.Transport.Insecure = true
		}
	}
	// Apply per-host TLS certificates from certs.d.
	if len(c.configs.CertsDirPaths) > 0 {
		if certs, err := remoteconfig.LoadCertsDirFromPaths(host, c.configs.CertsDirPaths); err == nil && certs != nil {
			certs.ApplyToTransport(&props.Transport)
		}
	}
	c.applyOverrides(props)
	return remote.NewRegistryWithProperties(props, c.builder)
}

// Generic returns a GenericClient for low-level OCI operations
func (c *Client) Generic() *GenericClient {
	return NewGenericClient(c)
}

// ClientOptDebug returns a function that sets the debug setting on client options set
func ClientOptDebug(debug bool) ClientOption {
	return func(client *Client) {
		client.debug = debug
	}
}

// ClientOptEnableCache returns a function that sets the enableCache setting on a client options set
func ClientOptEnableCache(enableCache bool) ClientOption {
	return func(client *Client) {
		client.enableCache = enableCache
	}
}

// ClientOptBasicAuth returns a function that sets the username and password setting on client options set
func ClientOptBasicAuth(username, password string) ClientOption {
	return func(client *Client) {
		client.username = username
		client.password = password
	}
}

// ClientOptWriter returns a function that sets the writer setting on client options set
func ClientOptWriter(out io.Writer) ClientOption {
	return func(client *Client) {
		client.out = out
	}
}

// ClientOptAuthorizer returns a function that sets the authorizer setting on a client options set. This
// can be used to override the default authorization mechanism.
//
// Depending on the use-case you may need to set both ClientOptAuthorizer and ClientOptRegistryAuthorizer.
func ClientOptAuthorizer(authorizer auth.Client) ClientOption {
	return func(client *Client) {
		client.authorizer = &authorizer
		client.customHTTPClient = true
	}
}

// ClientOptRegistryAuthorizer returns a function that sets the registry authorizer setting on a client options set. This
// can be used to override the default authorization mechanism.
//
// Depending on the use-case you may need to set both ClientOptAuthorizer and ClientOptRegistryAuthorizer.
func ClientOptRegistryAuthorizer(registryAuthorizer RemoteClient) ClientOption {
	return func(client *Client) {
		client.registryAuthorizer = registryAuthorizer
	}
}

// ClientOptCredentialsFile returns a function that sets the credentialsFile setting on a client options set
func ClientOptCredentialsFile(credentialsFile string) ClientOption {
	return func(client *Client) {
		client.credentialsFile = credentialsFile
	}
}

// ClientOptHTTPClient returns a function that sets the HTTP client for the registry client.
// When a custom HTTP client is provided, the legacy repository creation path is used so
// that TLS configuration in the custom transport is preserved.
func ClientOptHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = httpClient
		client.customHTTPClient = true
	}
}

// ClientOptPlainHTTP returns a function that enables plain HTTP (non-TLS)
// communication for the registry client.
func ClientOptPlainHTTP() ClientOption {
	return func(c *Client) {
		c.plainHTTP = true
	}
}

// ClientOptPolicyEvaluator returns a function that sets a custom policy evaluator on the client.
func ClientOptPolicyEvaluator(e *policy.Evaluator) ClientOption {
	return func(c *Client) {
		c.policyEvaluator = e
	}
}

// ClientOptSignatureVerification returns a function that enables or disables
// GPG/simple-signing signature verification via registries.d lookaside storage.
func ClientOptSignatureVerification(enabled bool) ClientOption {
	return func(c *Client) {
		c.signatureVerification = enabled
	}
}

// ClientOptConfigOptions returns a function that overrides default config file paths.
func ClientOptConfigOptions(o ConfigOptions) ClientOption {
	return func(c *Client) {
		c.configOptions = o
	}
}

type (
	// LoginOption allows specifying various settings on login
	LoginOption func(*loginOperation)

	loginOperation struct {
		host   string
		client *Client
	}
)

// warnIfHostHasPath checks if the host contains a repository path and logs a warning if it does.
// Returns true if the host contains a path component (i.e., contains a '/').
func warnIfHostHasPath(host string) bool {
	if strings.Contains(host, "/") {
		registryHost := strings.Split(host, "/")[0]
		slog.Warn("registry login currently only supports registry hostname, not a repository path", "host", host, "suggested", registryHost)
		return true
	}
	return false
}

// Login authenticates the client with a remote OCI registry using the provided host and options.
func (c *Client) Login(host string, options ...LoginOption) error {
	for _, option := range options {
		option(&loginOperation{host, c})
	}

	warnIfHostHasPath(host)

	// Determine canonical host from registries.conf Location rewrite.
	// We use the original (alias) host for newRegistry so its transport
	// settings (Insecure, certs.d) are preserved, then redirect the
	// authentication endpoint and credential key to the canonical host.
	canonicalHost := host
	if c.configs != nil && c.configs.RegistriesConfig != nil {
		if regCfg := c.configs.RegistriesConfig.FindRegistry(host); regCfg != nil && regCfg.Location != "" {
			canonicalHost = regCfg.Location
		}
	}

	reg, err := c.newRegistry(host)
	if err != nil {
		return err
	}
	reg.Reference.Registry = canonicalHost

	cred := credentials.Credential{Username: c.username, Password: c.password}
	ctx := context.Background()
	if err := remote.Login(ctx, c.credentialsStore, reg, cred); err != nil {
		return fmt.Errorf("authenticating to %q: %w", canonicalHost, err)
	}

	_, _ = fmt.Fprintln(c.out, "Login Succeeded")
	return nil
}

// LoginOptBasicAuth returns a function that sets the username/password settings on login
func LoginOptBasicAuth(username string, password string) LoginOption {
	return func(o *loginOperation) {
		o.client.username = username
		o.client.password = password
	}
}

// LoginOptPlainText returns a function that enables plaintext (HTTP) login
// instead of HTTPS for the registry client.
func LoginOptPlainText(isPlainText bool) LoginOption {
	return func(o *loginOperation) {
		o.client.plainHTTP = isPlainText
	}
}

func ensureTLSConfig(client *auth.Client, setConfig *tls.Config) (*tls.Config, error) {
	var transport *http.Transport

	switch t := client.Client.Transport.(type) {
	case *http.Transport:
		transport = t
	case *retry.Transport:
		switch t := t.Base.(type) {
		case *http.Transport:
			transport = t
		case *LoggingTransport:
			switch t := t.RoundTripper.(type) {
			case *http.Transport:
				transport = t
			}
		}
	}

	if transport == nil {
		// we don't know how to access the http.Transport, most likely the
		// auth.Client.Client was provided by API user
		return nil, fmt.Errorf("unable to access TLS client configuration, the provided HTTP Transport is not supported, given: %T", client.Client.Transport)
	}

	switch {
	case setConfig != nil:
		transport.TLSClientConfig = setConfig
	case transport.TLSClientConfig == nil:
		transport.TLSClientConfig = &tls.Config{}
	}

	return transport.TLSClientConfig, nil
}

// LoginOptInsecure returns a function that sets the insecure setting on login
func LoginOptInsecure(insecure bool) LoginOption {
	return func(o *loginOperation) {
		o.client.insecure = insecure
		// Also update the authorizer transport for the legacy path (customHTTPClient=true).
		tlsConfig, err := ensureTLSConfig(o.client.authorizer, nil)
		if err != nil {
			panic(err)
		}
		tlsConfig.InsecureSkipVerify = insecure
	}
}

// LoginOptTLSClientConfig returns a function that sets the TLS settings on login.
func LoginOptTLSClientConfig(certFile, keyFile, caFile string) LoginOption {
	return func(o *loginOperation) {
		if (certFile == "" || keyFile == "") && caFile == "" {
			return
		}
		// Set file path fields for the config-driven path.
		if certFile != "" && keyFile != "" {
			o.client.certFile = certFile
			o.client.keyFile = keyFile
		}
		if caFile != "" {
			o.client.caFile = caFile
		}
		// Also update the authorizer transport for the legacy path (customHTTPClient=true).
		tlsConfig, err := ensureTLSConfig(o.client.authorizer, nil)
		if err != nil {
			panic(err)
		}
		if certFile != "" && keyFile != "" {
			authCert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				panic(err)
			}
			tlsConfig.Certificates = []tls.Certificate{authCert}
		}
		if caFile != "" {
			certPool := x509.NewCertPool()
			ca, err := os.ReadFile(caFile)
			if err != nil {
				panic(err)
			}
			if !certPool.AppendCertsFromPEM(ca) {
				panic(fmt.Errorf("unable to parse CA file: %q", caFile))
			}
			tlsConfig.RootCAs = certPool
		}
	}
}

// LoginOptTLSClientConfigFromConfig returns a function that sets the TLS settings on login
// receiving the configuration in memory rather than from files.
func LoginOptTLSClientConfigFromConfig(conf *tls.Config) LoginOption {
	return func(o *loginOperation) {
		_, err := ensureTLSConfig(o.client.authorizer, conf)
		if err != nil {
			panic(err)
		}
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

	// Apply the same Location rewrite used by Login so the correct credential is removed.
	if c.configs != nil && c.configs.RegistriesConfig != nil {
		if regCfg := c.configs.RegistriesConfig.FindRegistry(host); regCfg != nil && regCfg.Location != "" {
			host = regCfg.Location
		}
	}

	if err := remote.Logout(context.Background(), c.credentialsStore, host); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(c.out, "Removing login credentials for %s\n", host)
	return nil
}

type (
	// PullOption allows specifying various settings on pull
	PullOption func(*pullOperation)

	// PullResult is the result returned upon successful pull.
	PullResult struct {
		Manifest *DescriptorPullSummary         `json:"manifest"`
		Config   *DescriptorPullSummary         `json:"config"`
		Chart    *DescriptorPullSummaryWithMeta `json:"chart"`
		Prov     *DescriptorPullSummary         `json:"prov"`
		Ref      string                         `json:"ref"`
	}

	DescriptorPullSummary struct {
		Data   []byte `json:"-"`
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	}

	DescriptorPullSummaryWithMeta struct {
		DescriptorPullSummary
		Meta *chart.Metadata `json:"meta"`
	}

	pullOperation struct {
		withChart         bool
		withProv          bool
		ignoreMissingProv bool
	}
)

// processChartPull handles chart-specific processing of a generic pull result
func (c *Client) processChartPull(genericResult *GenericPullResult, operation *pullOperation) (*PullResult, error) {
	var err error

	// Chart-specific validation
	minNumDescriptors := 1 // 1 for the config
	if operation.withChart {
		minNumDescriptors++
	}
	if operation.withProv && !operation.ignoreMissingProv {
		minNumDescriptors++
	}

	numDescriptors := len(genericResult.Descriptors)
	if numDescriptors < minNumDescriptors {
		return nil, fmt.Errorf("manifest does not contain minimum number of descriptors (%d), descriptors found: %d",
			minNumDescriptors, numDescriptors)
	}

	// Find chart-specific descriptors
	var configDescriptor *ocispec.Descriptor
	var chartDescriptor *ocispec.Descriptor
	var provDescriptor *ocispec.Descriptor

	for _, descriptor := range genericResult.Descriptors {
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
			_, _ = fmt.Fprintf(c.out, "Warning: chart media type %s is deprecated\n", LegacyChartLayerMediaType)
		}
	}

	// Chart-specific validation
	if configDescriptor == nil {
		return nil, fmt.Errorf("could not load config with mediatype %s", ConfigMediaType)
	}
	if operation.withChart && chartDescriptor == nil {
		return nil, fmt.Errorf("manifest does not contain a layer with mediatype %s",
			ChartLayerMediaType)
	}

	var provMissing bool
	if operation.withProv && provDescriptor == nil {
		if operation.ignoreMissingProv {
			provMissing = true
		} else {
			return nil, fmt.Errorf("manifest does not contain a layer with mediatype %s",
				ProvLayerMediaType)
		}
	}

	// Build chart-specific result
	result := &PullResult{
		Manifest: &DescriptorPullSummary{
			Digest: genericResult.Manifest.Digest.String(),
			Size:   genericResult.Manifest.Size,
		},
		Config: &DescriptorPullSummary{
			Digest: configDescriptor.Digest.String(),
			Size:   configDescriptor.Size,
		},
		Chart: &DescriptorPullSummaryWithMeta{},
		Prov:  &DescriptorPullSummary{},
		Ref:   genericResult.Ref,
	}

	// Fetch data using generic client
	genericClient := c.Generic()

	result.Manifest.Data, err = genericClient.GetDescriptorData(genericResult.MemoryStore, genericResult.Manifest)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve blob with digest %s: %w", genericResult.Manifest.Digest, err)
	}

	result.Config.Data, err = genericClient.GetDescriptorData(genericResult.MemoryStore, *configDescriptor)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve blob with digest %s: %w", configDescriptor.Digest, err)
	}

	if err := json.Unmarshal(result.Config.Data, &result.Chart.Meta); err != nil {
		return nil, err
	}

	if operation.withChart {
		result.Chart.Data, err = genericClient.GetDescriptorData(genericResult.MemoryStore, *chartDescriptor)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve blob with digest %s: %w", chartDescriptor.Digest, err)
		}
		result.Chart.Digest = chartDescriptor.Digest.String()
		result.Chart.Size = chartDescriptor.Size
	}

	if operation.withProv && !provMissing {
		result.Prov.Data, err = genericClient.GetDescriptorData(genericResult.MemoryStore, *provDescriptor)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve blob with digest %s: %w", provDescriptor.Digest, err)
		}
		result.Prov.Digest = provDescriptor.Digest.String()
		result.Prov.Size = provDescriptor.Size
	}

	_, _ = fmt.Fprintf(c.out, "Pulled: %s\n", result.Ref)
	_, _ = fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)

	if strings.Contains(result.Ref, "_") {
		_, _ = fmt.Fprintf(c.out, "%s contains an underscore.\n", result.Ref)
		_, _ = fmt.Fprint(c.out, registryUnderscoreMessage+"\n")
	}

	return result, nil
}

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

	// Build allowed media types for chart pull
	allowedMediaTypes := []string{
		ocispec.MediaTypeImageIndex,
		ocispec.MediaTypeImageManifest,
		ConfigMediaType,
	}
	if operation.withChart {
		allowedMediaTypes = append(allowedMediaTypes, ChartLayerMediaType, LegacyChartLayerMediaType)
	}
	if operation.withProv {
		allowedMediaTypes = append(allowedMediaTypes, ProvLayerMediaType)
	}

	// Use generic client for the pull operation
	genericClient := c.Generic()
	genericResult, err := genericClient.PullGeneric(ref, GenericPullOptions{
		AllowedMediaTypes: allowedMediaTypes,
	})
	if err != nil {
		return nil, err
	}

	// Process the result with chart-specific logic
	return c.processChartPull(genericResult, operation)
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
		provData     []byte
		strictMode   bool
		creationTime string
	}
)

// Push uploads a chart to a registry.
func (c *Client) Push(data []byte, ref string, options ...PushOption) (*PushResult, error) {
	parsedRef, err := newReference(ref)
	if err != nil {
		return nil, err
	}

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

	ctx := context.Background()

	memoryStore := memory.New()
	chartDescriptor, err := oras.PushBytes(ctx, memoryStore, ChartLayerMediaType, data)
	if err != nil {
		return nil, err
	}

	configData, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}

	configDescriptor, err := oras.PushBytes(ctx, memoryStore, ConfigMediaType, configData)
	if err != nil {
		return nil, err
	}

	layers := []ocispec.Descriptor{chartDescriptor}
	var provDescriptor ocispec.Descriptor
	if operation.provData != nil {
		provDescriptor, err = oras.PushBytes(ctx, memoryStore, ProvLayerMediaType, operation.provData)
		if err != nil {
			return nil, err
		}

		layers = append(layers, provDescriptor)
	}

	// sort layers for determinism, similar to how ORAS v1 does it
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Digest < layers[j].Digest
	})

	ociAnnotations := generateOCIAnnotations(meta, operation.creationTime)

	manifestDescriptor, err := c.tagManifest(ctx, memoryStore, configDescriptor,
		layers, ociAnnotations, parsedRef)
	if err != nil {
		return nil, err
	}

	repository, err := c.newRepository(parsedRef.String())
	if err != nil {
		return nil, err
	}

	manifestDescriptor, err = oras.ExtendedCopy(ctx, memoryStore, parsedRef.String(), repository, parsedRef.String(), oras.DefaultExtendedCopyOptions)
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
			Digest: manifestDescriptor.Digest.String(),
			Size:   manifestDescriptor.Size,
		},
		Config: &descriptorPushSummary{
			Digest: configDescriptor.Digest.String(),
			Size:   configDescriptor.Size,
		},
		Chart: chartSummary,
		Prov:  &descriptorPushSummary{}, // prevent nil references
		Ref:   parsedRef.String(),
	}
	if operation.provData != nil {
		result.Prov = &descriptorPushSummary{
			Digest: provDescriptor.Digest.String(),
			Size:   provDescriptor.Size,
		}
	}
	_, _ = fmt.Fprintf(c.out, "Pushed: %s\n", result.Ref)
	_, _ = fmt.Fprintf(c.out, "Digest: %s\n", result.Manifest.Digest)
	if strings.Contains(parsedRef.Tag, "_") {
		_, _ = fmt.Fprintf(c.out, "%s contains an underscore.\n", result.Ref)
		_, _ = fmt.Fprint(c.out, registryUnderscoreMessage+"\n")
	}

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

// PushOptCreationTime returns a function that sets the creation time
func PushOptCreationTime(creationTime string) PushOption {
	return func(operation *pushOperation) {
		operation.creationTime = creationTime
	}
}

// Tags provides a sorted list all semver compliant tags for a given repository
func (c *Client) Tags(ref string) ([]string, error) {
	parsedReference, err := properties.NewReference(ref)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	repository, err := c.newRepository(parsedReference.String())
	if err != nil {
		return nil, err
	}

	var tagVersions []*semver.Version
	err = repository.Tags(ctx, "", func(tags []string) error {
		for _, tag := range tags {
			// Change underscore (_) back to plus (+) for Helm
			// See https://github.com/helm/helm/issues/10166
			tagVersion, err := semver.StrictNewVersion(strings.ReplaceAll(tag, "_", "+"))
			if err == nil {
				tagVersions = append(tagVersions, tagVersion)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort the collection
	sort.Sort(sort.Reverse(semver.Collection(tagVersions)))

	tags := make([]string, len(tagVersions))

	for iTv, tv := range tagVersions {
		tags[iTv] = tv.String()
	}

	return tags, nil

}

// Resolve a reference to a descriptor.
func (c *Client) Resolve(ref string) (desc ocispec.Descriptor, err error) {
	remoteRepository, err := c.newRepository(ref)
	if err != nil {
		return desc, err
	}

	parsedReference, err := newReference(ref)
	if err != nil {
		return desc, err
	}

	ctx := context.Background()
	parsedString := parsedReference.String()
	return remoteRepository.Resolve(ctx, parsedString)
}

// ValidateReference for path and version
func (c *Client) ValidateReference(ref, version string, u *url.URL) (string, *url.URL, error) {
	var tag string

	registryReference, err := newReference(u.Host + u.Path)
	if err != nil {
		return "", nil, err
	}

	if version == "" {
		// Use OCI URI tag as default
		version = registryReference.Tag
	} else {
		if registryReference.Tag != "" && registryReference.Tag != version {
			return "", nil, fmt.Errorf("chart reference and version mismatch: %s is not %s", version, registryReference.Tag)
		}
	}

	if registryReference.Digest != "" {
		if version == "" {
			// Install by digest only
			return "", u, nil
		}
		u.Path = fmt.Sprintf("%s@%s", registryReference.Repository, registryReference.Digest)

		// Validate the tag if it was specified
		path := registryReference.Registry + "/" + registryReference.Repository + ":" + version
		desc, err := c.Resolve(path)
		if err != nil {
			// The resource does not have to be tagged when digest is specified
			return "", u, nil
		}
		if desc.Digest.String() != registryReference.Digest {
			return "", nil, fmt.Errorf("chart reference digest mismatch: %s is not %s", desc.Digest.String(), registryReference.Digest)
		}
		return registryReference.Digest, u, nil
	}

	// Evaluate whether an explicit version has been provided. Otherwise, determine version to use
	_, errSemVer := semver.NewVersion(version)
	if errSemVer == nil {
		tag = version
	} else {
		// Retrieve list of repository tags
		tags, err := c.Tags(strings.TrimPrefix(ref, OCIScheme+"://"))
		if err != nil {
			return "", nil, err
		}
		if len(tags) == 0 {
			return "", nil, fmt.Errorf("unable to locate any tags in provided repository: %s", ref)
		}

		// Determine if version provided
		// If empty, try to get the highest available tag
		// If exact version, try to find it
		// If semver constraint string, try to find a match
		tag, err = GetTagMatchingVersionOrConstraint(tags, version)
		if err != nil {
			return "", nil, err
		}
	}

	u.Path = fmt.Sprintf("%s:%s", registryReference.Repository, tag)
	// desc, err := c.Resolve(u.Path)

	return "", u, err
}

// tagManifest prepares and tags a manifest in memory storage
func (c *Client) tagManifest(ctx context.Context, memoryStore *memory.Store,
	configDescriptor ocispec.Descriptor, layers []ocispec.Descriptor,
	ociAnnotations map[string]string, parsedRef reference) (ocispec.Descriptor, error) {

	manifest := ocispec.Manifest{
		Versioned:   specs.Versioned{SchemaVersion: 2},
		Config:      configDescriptor,
		Layers:      layers,
		Annotations: ociAnnotations,
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return oras.TagBytes(ctx, memoryStore, ocispec.MediaTypeImageManifest,
		manifestData, parsedRef.String())
}
