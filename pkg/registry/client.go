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
		// containers holds the container ecosystem config stack and the
		// settings that control how it is loaded and enforced.
		containers containersConfig
		// tls holds TLS settings from client and login options, applied on
		// top of registries.conf and certs.d.
		tls              tlsOverrides
		builder          *remote.ClientBuilder
		customHTTPClient bool // true when ClientOptHTTPClient, ClientOptAuthorizer or LoginOptTLSClientConfigFromConfig was used
	}

	// containersConfig holds the container ecosystem config stack: Docker
	// config.json, containers auth.json, registries.conf, certs.d, policy.json
	// and registries.d.
	containersConfig struct {
		// enabled loads the config stack; Docker config.json credentials are
		// used either way.
		enabled               bool
		policyEnforcement     bool
		signatureVerification bool
		// registriesConfigPath overrides the registries.conf path; empty means
		// the container ecosystem default search locations are used. Test-only.
		registriesConfigPath string
		// policyConfigPath overrides the containers-policy.json path; empty
		// means the default search locations are used. Test-only.
		policyConfigPath string

		// configs is the loaded config stack; it is empty when the stack is
		// disabled or failed to load.
		configs         *remoteconfig.Configs
		policyEvaluator *policy.Evaluator
		// loadErr is the error from loading the config stack, returned by
		// registry operations so that NewClient itself does not fail.
		loadErr error
	}

	// tlsOverrides holds TLS settings from client and login options.
	tlsOverrides struct {
		insecure bool
		certFile string
		keyFile  string
		caFile   string
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

	// Without the container ecosystem config stack, configs stays empty (no
	// mirrors, rewrites, certs.d or policy) and Docker config.json is the only
	// fallback for Helm's own credentials, as it was with oras-go v2.
	configs := &remoteconfig.Configs{}
	var credStore credentials.Store = helmStore
	if !client.containers.enabled {
		if dockerStore, err := credentials.NewStoreFromDocker(storeOptions); err == nil {
			credStore = credentials.NewStoreWithFallbacks(helmStore, dockerStore)
		}
	} else {
		// Load the full container ecosystem config stack: Docker config.json,
		// containers auth.json, registries.conf, policy.json, certs.d, registries.d.
		// Missing files are silently skipped.
		// Only registries.conf and policy.json are overridable, and only by tests
		// (empty means default search); all other config files always resolve to
		// their default locations.
		loaderOpts := remoteconfig.LoadConfigsOptions{
			RegistriesConfigPath: client.containers.registriesConfigPath,
			PolicyConfigPath:     client.containers.policyConfigPath,
		}
		// A config file that exists but cannot be parsed must not break commands
		// that never contact a registry, so the error is kept and returned by the
		// first registry operation instead.
		loaded, err := remoteconfig.LoadConfigsWithOptions(loaderOpts)
		if err != nil {
			client.containers.loadErr = fmt.Errorf("failed to load registry configurations: %w", err)
		} else {
			configs = loaded
		}

		// policy.json is written for container images, so it is only enforced on
		// charts when the caller opts in.
		if (client.containers.policyEnforcement || client.containers.signatureVerification) && configs.PolicyConfig != nil {
			evaluator, err := newPolicyEvaluator(configs, client.containers.signatureVerification)
			if err != nil {
				return nil, fmt.Errorf("failed to build policy evaluator: %w", err)
			}
			client.containers.policyEvaluator = evaluator
		}

		// Build the combined credential store for read operations:
		//   1. Helm's own credentials file (highest priority, stores login results)
		//   2. Docker config.json + containers auth.json from the full config stack
		if configStore, err := configs.CredentialStore(storeOptions); err == nil {
			credStore = credentials.NewStoreWithFallbacks(helmStore, configStore)
		}
	}
	client.containers.configs = configs

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
	// Start from a transport Helm owns so TLS settings from certs.d and CLI
	// flags apply even when http.DefaultTransport has been replaced.
	builder.BaseTransport = defaultTransport()
	builder.UserAgent = version.GetUserAgent()
	builder.Logger = logger
	builder.PolicyEvaluator = client.containers.policyEvaluator
	// The builder's default factory allocates a new cache for every client
	// it builds, and a client is built per operation, so share one cache to
	// reuse tokens across operations. A nil cache disables caching.
	var cache auth.Cache
	if client.enableCache {
		cache = auth.NewCache()
	}
	builder.CacheFactory = func(string) auth.Cache { return cache }
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
	if c.tls.insecure {
		props.Transport.Insecure = true
	}
	if c.tls.certFile != "" && c.tls.keyFile != "" {
		props.Transport.Cert = c.tls.certFile
		props.Transport.Key = c.tls.keyFile
	}
	if c.tls.caFile != "" {
		props.Transport.CACerts = append(props.Transport.CACerts, c.tls.caFile)
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
	if c.containers.loadErr != nil {
		return nil, c.containers.loadErr
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
		repo.Registry.Policy = c.containers.policyEvaluator
		return repo, nil
	}
	// Config-driven path: resolve properties from registries.conf, certs.d, etc.
	props, err := c.containers.configs.RegistryProperties(ref)
	if err != nil {
		return nil, err
	}
	c.applyOverrides(props)
	repo, err := c.builder.NewRepository(props)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// newPolicyEvaluator builds the policy.json evaluator. With signature
// verification enabled it also verifies signedBy requirements against the
// registries.d lookaside storage; the verifier resolves the lookaside URL for
// each repository, so one evaluator serves every repository.
func newPolicyEvaluator(configs *remoteconfig.Configs, signatureVerification bool) (*policy.Evaluator, error) {
	var opts []policy.EvaluatorOption
	if signatureVerification && configs.RegistriesDConfig != nil {
		opts = append(opts, policy.WithSignedByVerifier(signature.NewSignedByVerifierFromConfig(configs.RegistriesDConfig)))
	}
	return configs.PolicyEvaluator(opts...)
}

// loginPlaceholderRepository is appended to a registry host to look up its
// registries.conf and certs.d settings, because Configs.RegistryProperties only
// accepts a reference that names a repository.
const loginPlaceholderRepository = "helm-registry-login"

// newRegistry creates a configured remote.Registry for Login.
//
// ref is a registry host optionally followed by a namespace path. The
// registries.conf Location rewrite is applied to the full input, since a rule
// may be scoped to a path (e.g. "alias.example/team"), so the returned
// registry's Reference names the canonical registry and namespace the
// credential is verified against and stored under.
func (c *Client) newRegistry(ref string) (*remote.Registry, error) {
	if c.containers.loadErr != nil {
		return nil, c.containers.loadErr
	}
	if c.customHTTPClient {
		// Legacy path: use c.authorizer directly. registryAuthorizer is not
		// used here because remote.Login only accepts an *auth.Client.
		target := ref
		if rc := c.containers.configs.RegistriesConfig; rc != nil {
			target = rc.RewriteReference(ref)
		}
		host, namespace, _ := strings.Cut(target, "/")
		reg, err := remote.NewRegistry(host)
		if err != nil {
			return nil, err
		}
		reg.Reference.Repository = namespace
		reg.PlainHTTP = c.plainHTTP
		reg.Client = c.authorizer
		return reg, nil
	}
	// Config-driven path: resolve the settings the way pull and push do, from
	// registries.conf (aliases, blocking, rewrites, insecure, token flow,
	// blocked realm prefixes) and certs.d, then apply CLI overrides.
	props, err := c.loginProperties(ref)
	if err != nil {
		return nil, err
	}
	c.applyOverrides(props)
	reg, err := c.builder.NewRegistry(props)
	if err != nil {
		return nil, err
	}
	// The builder keeps only the registry name; restore the full reference so
	// remote.Login stores a namespaced credential and rejects a target that
	// carries a tag or digest.
	reg.Reference = props.Reference
	return reg, nil
}

// loginProperties resolves the registry properties for a login target through
// Configs.RegistryProperties. A host-only target is resolved with a
// placeholder repository, which is then removed from the result.
//
// Source, which records the pre-rewrite location so a repository can re-root
// references naming it, is cleared: a login has no references to re-root.
func (c *Client) loginProperties(ref string) (*properties.Registry, error) {
	if strings.Contains(ref, "/") {
		props, err := c.containers.configs.RegistryProperties(ref)
		if err != nil {
			return nil, err
		}
		props.Source = properties.Reference{}
		return props, nil
	}
	probe := ref + "/" + loginPlaceholderRepository
	if rc := c.containers.configs.RegistriesConfig; rc != nil {
		if rc.IsBlocked(ref) {
			return nil, fmt.Errorf("%w: %s", remoteconfig.ErrRegistryBlocked, ref)
		}
		// The placeholder must select the same registries.conf entry as the
		// host itself, or a path-scoped rule would apply to a host login.
		if rc.FindRegistry(probe) != rc.FindRegistry(ref) {
			return nil, fmt.Errorf("registries.conf has a rule for %q, which Helm uses to look up the settings for %q", probe, ref)
		}
	}
	props, err := c.containers.configs.RegistryProperties(probe)
	if err != nil {
		return nil, err
	}
	namespace := strings.TrimSuffix(props.Reference.Repository, loginPlaceholderRepository)
	props.Reference.Repository = strings.TrimSuffix(namespace, "/")
	props.Source = properties.Reference{}
	return props, nil
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
// ClientOptCredentialsFile to configure authorization.
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

// ClientOptContainersConfig returns a function that enables or disables
// loading the container ecosystem configuration: containers auth.json,
// registries.conf (mirrors and location rewrites), certs.d, policy.json and
// registries.d. It is disabled by default, in which case credentials come from
// the Helm credentials file and Docker config.json only. Policy enforcement and
// signature verification have no effect unless it is enabled.
func ClientOptContainersConfig(enabled bool) ClientOption {
	return func(c *Client) {
		c.containers.enabled = enabled
	}
}

// ClientOptPolicyEnforcement returns a function that enables or disables
// enforcement of the containers-policy.json trust policy on registry
// operations. It is disabled by default because the policy is written for
// container images, and its signedBy and sigstoreSigned requirements reject
// charts unless signature verification is also enabled.
func ClientOptPolicyEnforcement(enabled bool) ClientOption {
	return func(c *Client) {
		c.containers.policyEnforcement = enabled
	}
}

// ClientOptSignatureVerification returns a function that enables or disables
// GPG/simple-signing signature verification via registries.d lookaside storage.
// Enabling it also enforces the containers-policy.json trust policy.
func ClientOptSignatureVerification(enabled bool) ClientOption {
	return func(c *Client) {
		c.containers.signatureVerification = enabled
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
			c.tls.certFile = certFile
			c.tls.keyFile = keyFile
		}
		c.tls.caFile = caFile
	}
}

// ClientOptInsecureSkipVerifyTLS returns a function that disables verification
// of the registry's TLS certificate.
func ClientOptInsecureSkipVerifyTLS(insecure bool) ClientOption {
	return func(c *Client) {
		c.tls.insecure = insecure
	}
}

// withRegistriesConfigPath overrides the registries.conf path. It is unexported
// because the only consumer is hermetic tests; production always uses the
// container ecosystem default search locations.
func withRegistriesConfigPath(path string) ClientOption {
	return func(c *Client) {
		c.containers.registriesConfigPath = path
	}
}

// withPolicyConfigPath overrides the containers-policy.json path. Like
// withRegistriesConfigPath, it exists only for hermetic tests.
func withPolicyConfigPath(path string) ClientOption {
	return func(c *Client) {
		c.containers.policyConfigPath = path
	}
}

type (
	// LoginOption allows specifying various settings on login
	LoginOption func(*loginOperation)

	loginOperation struct {
		host     string
		client   *Client
		username string
		password string
		err      error
	}
)

// Login authenticates the client with a remote OCI registry using the provided host and options.
func (c *Client) Login(host string, options ...LoginOption) error {
	op := &loginOperation{host: host, client: c, username: c.username, password: c.password}
	for _, option := range options {
		option(op)
	}
	if op.err != nil {
		return op.err
	}

	reg, err := c.newRegistry(host)
	if err != nil {
		return err
	}
	target := reg.Reference.Registry
	if reg.Reference.Repository != "" {
		target += "/" + reg.Reference.Repository
	}

	// remote.Login stores the credential under "host/namespace" when a
	// namespace is set, after validating the path and rejecting registries
	// such as Docker Hub that cannot use path-scoped credentials.
	cred := credentials.Credential{Username: op.username, Password: op.password}
	if err := remote.Login(context.Background(), c.credentialsStore, reg, cred); err != nil {
		return fmt.Errorf("authenticating to %q: %w", target, err)
	}

	_, _ = fmt.Fprintln(c.out, "Login Succeeded")
	return nil
}

// fail records an error from a LoginOption, which Login returns before
// contacting the registry. The first error is kept.
func (o *loginOperation) fail(err error) {
	if o.err == nil {
		o.err = err
	}
}

// LoginOptBasicAuth returns a function that sets the username/password settings on login
//
// The credential applies to this login only. Once stored, later operations
// read it from the credentials store, which scopes it to the registry and
// namespace it was stored under.
func LoginOptBasicAuth(username, password string) LoginOption {
	return func(o *loginOperation) {
		o.username = username
		o.password = password
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
		o.client.tls.insecure = insecure
		// Also update the authorizer transport for the legacy path (customHTTPClient=true).
		tlsConfig, err := ensureTLSConfig(o.client.authorizer, nil)
		if err != nil {
			o.fail(err)
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
		// Load and check every file before changing the client, so a failed
		// Login does not leave settings behind that break later operations.
		hasCert := certFile != "" && keyFile != ""
		var authCert tls.Certificate
		if hasCert {
			var err error
			if authCert, err = tls.LoadX509KeyPair(certFile, keyFile); err != nil {
				o.fail(fmt.Errorf("loading client certificate %q and key %q: %w", certFile, keyFile, err))
				return
			}
		}
		var certPool *x509.CertPool
		if caFile != "" {
			ca, err := os.ReadFile(caFile)
			if err != nil {
				o.fail(fmt.Errorf("reading CA file: %w", err))
				return
			}
			certPool = x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(ca) {
				o.fail(fmt.Errorf("unable to parse CA file: %q", caFile))
				return
			}
		}
		// The authorizer transport is used by the legacy path (customHTTPClient=true).
		tlsConfig, err := ensureTLSConfig(o.client.authorizer, nil)
		if err != nil {
			o.fail(err)
			return
		}

		// The file paths are used by the config-driven path.
		if hasCert {
			o.client.tls.certFile = certFile
			o.client.tls.keyFile = keyFile
			tlsConfig.Certificates = []tls.Certificate{authCert}
		}
		if certPool != nil {
			o.client.tls.caFile = caFile
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
		if _, err := ensureTLSConfig(o.client.authorizer, conf); err != nil {
			o.fail(err)
			return
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
	if c.containers.loadErr != nil {
		return c.containers.loadErr
	}

	// Apply the registries.conf Location rewrite to the full input, matching
	// Login, since a rule may be scoped to a repository path.
	if c.containers.configs != nil && c.containers.configs.RegistriesConfig != nil {
		host = c.containers.configs.RegistriesConfig.RewriteReference(host)
	}

	// remote.Logout removes the namespaced key for a "host/namespace" input,
	// matching where Login stored it.
	registryHost, namespacePath, _ := strings.Cut(host, "/")
	resource := properties.Resource{Registry: registryHost, Path: namespacePath}
	if err := remote.Logout(context.Background(), c.credentialsStore, resource); err != nil {
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
		SchemaVersion: 2,
		Config:        configDescriptor,
		Layers:        layers,
		Annotations:   ociAnnotations,
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
