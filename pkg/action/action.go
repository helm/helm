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

package action

import (
	"fmt"
	"os"
	"path"
	"regexp"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/time"
)

// Timestamper is a function capable of producing a timestamp.Timestamper.
//
// By default, this is a time.Time function from the Helm time package. This can
// be overridden for testing though, so that timestamps are predictable.
var Timestamper = time.Now

var (
	// errMissingChart indicates that a chart was not provided.
	errMissingChart = errors.New("no chart provided")
	// errMissingRelease indicates that a release (name) was not provided.
	errMissingRelease = errors.New("no release provided")
	// errInvalidRevision indicates that an invalid release revision number was provided.
	errInvalidRevision = errors.New("invalid release revision")
	// errInvalidName indicates that an invalid release name was provided
	errInvalidName = errors.New("invalid release name, must match regex ^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$ and the length must not longer than 53")
)

// ValidName is a regular expression for names.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
//
// We modified that. First, we added start and end delimiters. Second, we changed
// the final ? to + to require that the pattern match at least once. This modification
// prevents an empty string from matching.
var ValidName = regexp.MustCompile("^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$")

// Configuration injects the dependencies that all actions share.
type Configuration struct {
	// RESTClientGetter is an interface that loads Kubernetes clients.
	RESTClientGetter RESTClientGetter

	// Releases stores records of releases.
	Releases *storage.Storage

	// KubeClient is a Kubernetes API client.
	KubeClient kube.Interface

	// RegistryClient is a client for working with registries
	RegistryClient *registry.Client

	// Capabilities describes the capabilities of the Kubernetes cluster.
	Capabilities *chartutil.Capabilities

	Log func(string, ...interface{})
}

// RESTClientGetter gets the rest client
type RESTClientGetter interface {
	ToRESTConfig() (*rest.Config, error)
	ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error)
	ToRESTMapper() (meta.RESTMapper, error)
}

// DebugLog sets the logger that writes debug strings
type DebugLog func(format string, v ...interface{})

// capabilities builds a Capabilities from discovery information.
func (c *Configuration) getCapabilities() (*chartutil.Capabilities, error) {
	if c.Capabilities != nil {
		return c.Capabilities, nil
	}
	dc, err := c.RESTClientGetter.ToDiscoveryClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not get Kubernetes discovery client")
	}
	// force a discovery cache invalidation to always fetch the latest server version/capabilities.
	dc.Invalidate()
	kubeVersion, err := dc.ServerVersion()
	if err != nil {
		return nil, errors.Wrap(err, "could not get server version from Kubernetes")
	}
	// Issue #6361:
	// Client-Go emits an error when an API service is registered but unimplemented.
	// We trap that error here and print a warning. But since the discovery client continues
	// building the API object, it is correctly populated with all valid APIs.
	// See https://github.com/kubernetes/kubernetes/issues/72051#issuecomment-521157642
	apiVersions, err := GetVersionSet(dc)
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) {
			c.Log("WARNING: The Kubernetes server has an orphaned API service. Server reports: %s", err)
			c.Log("WARNING: To fix this, kubectl delete apiservice <service-name>")
		} else {
			return nil, errors.Wrap(err, "could not get apiVersions from Kubernetes")
		}
	}

	c.Capabilities = &chartutil.Capabilities{
		APIVersions: apiVersions,
		KubeVersion: chartutil.KubeVersion{
			Version: kubeVersion.GitVersion,
			Major:   kubeVersion.Major,
			Minor:   kubeVersion.Minor,
		},
	}
	return c.Capabilities, nil
}

// KubernetesClientSet creates a new kubernetes ClientSet based on the configuration
func (c *Configuration) KubernetesClientSet() (kubernetes.Interface, error) {
	conf, err := c.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate config for kubernetes client")
	}

	return kubernetes.NewForConfig(conf)
}

// Now generates a timestamp
//
// If the configuration has a Timestamper on it, that will be used.
// Otherwise, this will use time.Now().
func (c *Configuration) Now() time.Time {
	return Timestamper()
}

func (c *Configuration) releaseContent(name string, version int) (*release.Release, error) {
	if err := validateReleaseName(name); err != nil {
		return nil, errors.Errorf("releaseContent: Release name is invalid: %s", name)
	}

	if version <= 0 {
		return c.Releases.Last(name)
	}

	return c.Releases.Get(name, version)
}

// GetVersionSet retrieves a set of available k8s API versions
func GetVersionSet(client discovery.ServerResourcesInterface) (chartutil.VersionSet, error) {
	groups, resources, err := client.ServerGroupsAndResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return chartutil.DefaultVersionSet, errors.Wrap(err, "could not get apiVersions from Kubernetes")
	}

	// FIXME: The Kubernetes test fixture for cli appears to always return nil
	// for calls to Discovery().ServerGroupsAndResources(). So in this case, we
	// return the default API list. This is also a safe value to return in any
	// other odd-ball case.
	if len(groups) == 0 && len(resources) == 0 {
		return chartutil.DefaultVersionSet, nil
	}

	versionMap := make(map[string]interface{})
	versions := []string{}

	// Extract the groups
	for _, g := range groups {
		for _, gv := range g.Versions {
			versionMap[gv.GroupVersion] = struct{}{}
		}
	}

	// Extract the resources
	var id string
	var ok bool
	for _, r := range resources {
		for _, rl := range r.APIResources {

			// A Kind at a GroupVersion can show up more than once. We only want
			// it displayed once in the final output.
			id = path.Join(r.GroupVersion, rl.Kind)
			if _, ok = versionMap[id]; !ok {
				versionMap[id] = struct{}{}
			}
		}
	}

	// Convert to a form that NewVersionSet can use
	for k := range versionMap {
		versions = append(versions, k)
	}

	return chartutil.VersionSet(versions), nil
}

// recordRelease with an update operation in case reuse has been set.
func (c *Configuration) recordRelease(r *release.Release) {
	if err := c.Releases.Update(r); err != nil {
		c.Log("warning: Failed to update release %s: %s", r.Name, err)
	}
}

// Init initializes the action configuration
func (c *Configuration) Init(getter genericclioptions.RESTClientGetter, namespace, helmDriver string, log DebugLog) error {
	kc := kube.New(getter)
	kc.Log = log

	lazyClient := &lazyClient{
		namespace: namespace,
		clientFn:  kc.Factory.KubernetesClientSet,
	}

	var store *storage.Storage
	switch helmDriver {
	case "secret", "secrets", "":
		d := driver.NewSecrets(newSecretClient(lazyClient))
		d.Log = log
		store = storage.Init(d)
	case "configmap", "configmaps":
		d := driver.NewConfigMaps(newConfigMapClient(lazyClient))
		d.Log = log
		store = storage.Init(d)
	case "memory":
		var d *driver.Memory
		if c.Releases != nil {
			if mem, ok := c.Releases.Driver.(*driver.Memory); ok {
				// This function can be called more than once (e.g., helm list --all-namespaces).
				// If a memory driver was already initialized, re-use it but set the possibly new namespace.
				// We re-use it in case some releases where already created in the existing memory driver.
				d = mem
			}
		}
		if d == nil {
			d = driver.NewMemory()
		}
		d.SetNamespace(namespace)
		store = storage.Init(d)
	case "sql":
		d, err := driver.NewSQL(
			os.Getenv("HELM_DRIVER_SQL_CONNECTION_STRING"),
			log,
			namespace,
		)
		if err != nil {
			panic(fmt.Sprintf("Unable to instantiate SQL driver: %v", err))
		}
		store = storage.Init(d)
	default:
		// Not sure what to do here.
		panic("Unknown driver in HELM_DRIVER: " + helmDriver)
	}

	c.RESTClientGetter = getter
	c.KubeClient = kc
	c.Releases = store
	c.Log = log

	return nil
}
