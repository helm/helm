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

package registry

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
	authv2 "oras.land/oras-go/v2/registry/remote/auth"

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
		policyEnforcement     bool
		signatureVerification bool
		// configErr is the error from loading the config stack, returned by
		// registry operations so that NewClient itself does not fail.
		configErr error
		// registriesConfigPath overrides the registries.conf path; empty means
		// the container ecosystem default search locations are used. Test-only.
		registriesConfigPath string
		// policyConfigPath overrides the containers-policy.json path; empty
		// means the default search locations are used. Test-only.
		policyConfigPath string
		insecure         bool
		certFile         string
		keyFile          string
		caFile           string
		customHTTPClient bool // true when ClientOptHTTPClient, ClientOptAuthorizer or LoginOptTLSClientConfigFromConfig was used
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
	// Only registries.conf and policy.json are overridable, and only by tests
	// (empty means default search); all other config files always resolve to
	// their default locations.
	loaderOpts := remoteconfig.LoadConfigsOptions{
		RegistriesConfigPath: client.registriesConfigPath,
		PolicyConfigPath:     client.policyConfigPath,
	}
	// A config file that exists but cannot be parsed must not break commands
	// that never contact a registry, so the error is kept and returned by the
	// first registry operation instead.
	configs, err := remoteconfig.LoadConfigsWithOptions(loaderOpts)
	if err != nil {
		client.configErr = fmt.Errorf("failed to load registry configurations: %w", err)
		configs = &remoteconfig.Configs{}
	}
	client.configs = configs

	// policy.json is written for container images, so it is only enforced on
	// charts when the caller opts in.
	if (client.policyEnforcement || client.signatureVerification) && configs.PolicyConfig != nil {
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
	// The builder resolves credentials through remote.NewCredentialFunc, which
	// maps Docker Hub to its server address and walks the request's namespaces
	// from most to least specific, so namespaced logins apply.
	builder.CredentialStore = credStore
	builder.UserAgent = version.GetUserAgent()
	builder.Logger = logger
	builder.PolicyEvaluator = client.policyEvaluator
	if !client.enableCache {
		// A nil factory makes the builder allocate a cache, so return a nil
		// cache instead, which auth.Client treats as no caching.
		builder.CacheFactory = func(string) auth.Cache { return nil }
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
			authorizer.CredentialFunc = func(_ context.Context, _ properties.Resource) (credentials.Credential, error) {
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
	if c.username != "" && c.password != "" {
		props.Credential = credentials.Credential{Username: c.username, Password: c.password}
	}
}

// newRepository creates a configured remote.Repository for the given reference.
//
// When a custom HTTP client or authorizer was provided, the legacy path is used
// (direct assignment of c.authorizer). Otherwise, the full config-driven path is
// used: registry properties are resolved from registries.conf and certs.d, CLI
// overrides are applied, and the repository is built via ClientBuilder.NewRepository.
func (c *Client) newRepository(ref string) (*remote.Repository, error) {
	if c.configErr != nil {
		return nil, c.configErr
	}
	if c.customHTTPClient {
		// Legacy path: use c.authorizer directly (preserves custom TLS transport).
		// NewRepository rejects a reference carrying a tag or digest, so pass
		// only the repository portion.
		parsed, err := properties.NewReference(ref)
		if err != nil {
			return nil, err
		}
		repo, err := remote.NewRepository(parsed.Registry + "/" + parsed.Repository)
		if err != nil {
			return nil, err
		}
		repo.Registry.PlainHTTP = c.plainHTTP
		if c.registryAuthorizer != nil {
			repo.Registry.Client = c.registryAuthorizer
		} else {
			repo.Registry.Client = c.authorizer
		}
		repo.Registry.Policy = c.policyEvaluator
		return repo, nil
	}
	// Config-driven path: resolve properties from registries.conf, certs.d, etc.
	props, err := c.configs.RegistryProperties(ref)
	if err != nil {
		return nil, err
	}
	c.applyOverrides(props)
	repo, err := c.builder.NewRepository(props)
	if err != nil {
		return nil, err
	}
	// Replace the config-derived evaluator with one scoped to this repository's
	// registries.d signature storage.
	if c.signatureVerification && c.configs.RegistriesDConfig != nil && c.configs.PolicyConfig != nil {
		scope := props.Reference.Registry
		if props.Reference.Repository != "" {
			scope += "/" + props.Reference.Repository
		}
		verifier := signature.NewSignedByVerifierFromConfig(c.configs.RegistriesDConfig, scope)
		if verifier != nil {
			scopedEval, err := c.configs.PolicyEvaluator(policy.WithSignedByVerifier(verifier))
			if err != nil {
				return nil, fmt.Errorf("failed to build signature policy evaluator for %q: %w", scope, err)
			}
			repo.Registry.Policy = scopedEval
		}
	}
	return repo, nil
}

// newRegistry creates a configured remote.Registry for the given reference (used by Login).
//
// ref is a registry host optionally followed by a repository path; the path is
// only used to match path-scoped registries.conf rules.
func (c *Client) newRegistry(ref string) (*remote.Registry, error) {
	if c.configErr != nil {
		return nil, c.configErr
	}
	host, _, _ := strings.Cut(ref, "/")
	if c.customHTTPClient {
		// Legacy path: use c.authorizer directly. registryAuthorizer is not
		// used here because remote.Login only accepts an *auth.Client.
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
	// Configs.RegistryProperties cannot be used because it requires a
	// repository, so the host's registries.conf entry is applied here.
	props := properties.NewRegistryFromReference(properties.Reference{Registry: host})
	if rc := c.configs.RegistriesConfig; rc != nil {
		if rc.IsBlocked(ref) {
			return nil, fmt.Errorf("%w: %s", remoteconfig.ErrRegistryBlocked, ref)
		}
		if reg := rc.FindRegistry(ref); reg != nil {
			if reg.Insecure {
				props.Transport.Insecure = true
			}
			switch reg.TokenFlow {
			case "":
				// leave as TokenFlowDefault
			case "oauth2":
				props.Attributes.TokenFlow = properties.TokenFlowOAuth2
			case "distribution":
				props.Attributes.TokenFlow = properties.TokenFlowDistribution
			default:
				return nil, fmt.Errorf("invalid token-flow %q for registry %q: must be %q or %q",
					reg.TokenFlow, reg.Prefix, "oauth2", "distribution")
			}
		}
	}
	// Apply per-host TLS certificates from certs.d.
	if len(c.configs.CertsDirPaths) > 0 {
		if certs, err := remoteconfig.LoadCertsDirFromPaths(host, c.configs.CertsDirPaths); err == nil && certs != nil {
			certs.ApplyToTransport(&props.Transport)
		}
	}
	c.applyOverrides(props)
	return c.builder.NewRegistry(props)
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
//
// The oras-go v2 client is converted to an oras-go v3 client; see
// authClientFromV2 for how each field carries over.
//
// There is no replacement that takes an oras-go auth client. Exposing the
// oras-go type ties Helm's public API to an oras-go major version, so needs
// that this option covered should be met by Helm options for the specific
// feature (credentials, headers, TLS, HTTP client) rather than by exposing
// the implementation.
//
// Deprecated: use ClientOptHTTPClient, ClientOptBasicAuth or
// ClientOptCredentialsFile to configure authorization, or
// ClientOptRegistryAuthorizer to replace the client used for registry requests.
func ClientOptAuthorizer(authorizer authv2.Client) ClientOption {
	return func(client *Client) {
		client.authorizer = authClientFromV2(authorizer)
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

// ClientOptPolicyEnforcement returns a function that enables or disables
// enforcement of the containers-policy.json trust policy on registry
// operations. It is disabled by default because the policy is written for
// container images, and its signedBy and sigstoreSigned requirements reject
// charts unless signature verification is also enabled.
func ClientOptPolicyEnforcement(enabled bool) ClientOption {
	return func(c *Client) {
		c.policyEnforcement = enabled
	}
}

// ClientOptSignatureVerification returns a function that enables or disables
// GPG/simple-signing signature verification via registries.d lookaside storage.
// Enabling it also enforces the containers-policy.json trust policy.
func ClientOptSignatureVerification(enabled bool) ClientOption {
	return func(c *Client) {
		c.signatureVerification = enabled
	}
}

// ClientOptTLSClientConfig returns a function that sets the client certificate,
// key and CA bundle files used for TLS. Unlike passing a TLS transport through
// ClientOptHTTPClient, the files are applied on top of the registries.conf and
// certs.d settings, so those keep working. A certificate is only used when both
// certFile and keyFile are set.
func ClientOptTLSClientConfig(certFile, keyFile, caFile string) ClientOption {
	return func(c *Client) {
		if certFile != "" && keyFile != "" {
			c.certFile = certFile
			c.keyFile = keyFile
		}
		c.caFile = caFile
	}
}

// ClientOptInsecureSkipVerifyTLS returns a function that disables verification
// of the registry's TLS certificate.
func ClientOptInsecureSkipVerifyTLS(insecure bool) ClientOption {
	return func(c *Client) {
		c.insecure = insecure
	}
}

// withRegistriesConfigPath overrides the registries.conf path. It is unexported
// because the only consumer is hermetic tests; production always uses the
// container ecosystem default search locations.
func withRegistriesConfigPath(path string) ClientOption {
	return func(c *Client) {
		c.registriesConfigPath = path
	}
}

// withPolicyConfigPath overrides the containers-policy.json path. Like
// withRegistriesConfigPath, it exists only for hermetic tests.
func withPolicyConfigPath(path string) ClientOption {
	return func(c *Client) {
		c.policyConfigPath = path
	}
}

type (
	// LoginOption allows specifying various settings on login
	LoginOption func(*loginOperation)

	loginOperation struct {
		host   string
		client *Client
		err    error
	}
)

// noopStore is a credentials.Putter that performs no persistence. It is used
// during namespaced login to verify credentials with a registry ping without
// causing remote.Login to store credentials under the hostname-only key.
type noopStore struct{}

func (noopStore) Put(_ context.Context, _ string, _ credentials.Credential) error { return nil }

// Login authenticates the client with a remote OCI registry using the provided host and options.
func (c *Client) Login(host string, options ...LoginOption) error {
	op := &loginOperation{host: host, client: c}
	for _, option := range options {
		option(op)
	}
	if op.err != nil {
		return op.err
	}

	// Apply the registries.conf Location rewrite to the full input, since a
	// rule may be scoped to a repository path (e.g. "alias.example/team").
	target := host
	if c.configs != nil && c.configs.RegistriesConfig != nil {
		target = c.configs.RegistriesConfig.RewriteReference(host)
	}

	// Separate the canonical registry hostname from any namespace path.
	// e.g., "localhost:8000/myrepo" → canonicalHost="localhost:8000", namespacePath="myrepo"
	canonicalHost, namespacePath, hasNamespace := strings.Cut(target, "/")

	// We use the original (alias) reference for newRegistry so its transport
	// settings (Insecure, certs.d, token-flow, blocked) are preserved, then
	// redirect the authentication endpoint and credential key to the
	// canonical host.
	reg, err := c.newRegistry(host)
	if err != nil {
		return err
	}
	reg.Reference.Registry = canonicalHost

	cred := credentials.Credential{Username: c.username, Password: c.password}
	ctx := context.Background()

	if !hasNamespace {
		// Standard hostname-only login: verify and store under the hostname.
		if err := remote.Login(ctx, c.credentialsStore, reg, cred); err != nil {
			return fmt.Errorf("authenticating to %q: %w", canonicalHost, err)
		}
	} else {
		// Namespaced login: verify the credential against the hostname,
		// then store under the namespaced key only.
		if err := remote.Login(ctx, noopStore{}, reg, cred); err != nil {
			return fmt.Errorf("authenticating to %q: %w", canonicalHost, err)
		}
		namespacedKey := canonicalHost + "/" + namespacePath
		if err := c.credentialsStore.Put(ctx, namespacedKey, cred); err != nil {
			return fmt.Errorf("storing credentials for %q: %w", namespacedKey, err)
		}
	}

	_, _ = fmt.Fprintln(c.out, "Login Succeeded")
	return nil
}

// LoginOptBasicAuth returns a function that sets the username/password settings on login
func LoginOptBasicAuth(username, password string) LoginOption {
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
			if t, ok := t.RoundTripper.(*http.Transport); ok {
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

	// Idle connections were established under the previous TLS configuration.
	// Drop them so the settings being applied here take effect on the next
	// request instead of being bypassed by a pooled connection.
	transport.CloseIdleConnections()

	return transport.TLSClientConfig, nil
}

// LoginOptInsecure returns a function that sets the insecure setting on login
func LoginOptInsecure(insecure bool) LoginOption {
	return func(o *loginOperation) {
		o.client.insecure = insecure
		// Also update the authorizer transport for the legacy path (customHTTPClient=true).
		tlsConfig, err := ensureTLSConfig(o.client.authorizer, nil)
		if err != nil {
			o.err = err
			return
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
//
// The config-driven path builds its TLS configuration from files, so this option
// switches the client to the legacy path, which uses the authorizer's transport.
func LoginOptTLSClientConfigFromConfig(conf *tls.Config) LoginOption {
	return func(o *loginOperation) {
		_, err := ensureTLSConfig(o.client.authorizer, conf)
		if err != nil {
			panic(err)
		}
		o.client.customHTTPClient = true
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
	if c.configErr != nil {
		return c.configErr
	}

	// Apply the registries.conf Location rewrite to the full input, matching
	// Login, since a rule may be scoped to a repository path.
	if c.configs != nil && c.configs.RegistriesConfig != nil {
		host = c.configs.RegistriesConfig.RewriteReference(host)
	}

	ctx := context.Background()
	if registryHost, _, hasNamespace := strings.Cut(host, "/"); !hasNamespace {
		if err := remote.Logout(ctx, c.credentialsStore, properties.Resource{Registry: registryHost}); err != nil {
			return err
		}
	} else if err := c.credentialsStore.Delete(ctx, host); err != nil {
		// Namespaced credentials are stored under the full "host/namespace"
		// key by Login, so remove that exact key.
		return fmt.Errorf("failed to delete the credential for %s: %w", host, err)
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
		if !operation.ignoreMissingProv {
			return nil, fmt.Errorf("manifest does not contain a layer with mediatype %s",
				ProvLayerMediaType)
		}
		provMissing = true
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

	ctx = withScopeHint(ctx, repository, auth.ActionPull, auth.ActionPush)

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
	return remoteRepository.Resolve(ctx, parsedReference.String())
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
	} else if registryReference.Tag != "" && registryReference.Tag != version {
		return "", nil, fmt.Errorf("chart reference and version mismatch: %s is not %s", version, registryReference.Tag)
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
	ociAnnotations map[string]string, parsedRef reference,
) (ocispec.Descriptor, error) {
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

// withScopeHint hints the auth client to request a token covering all the given
// actions in a single request. Without this, pushing to a token-auth registry
// first requests a [pull] scope (which fails for a not-yet-existing repository
// path), making it hard to mint a valid token. Hinting [pull,push] up front
// produces a single correct token request.
func withScopeHint(ctx context.Context, repo *remote.Repository, actions ...string) context.Context {
	return auth.AppendRepositoryScope(ctx, repo.Reference(), actions...)
}
