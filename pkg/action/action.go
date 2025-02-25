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
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/postrender"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
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
	// errPending indicates that another instance of Helm is already applying an operation on a release.
	errPending = errors.New("another operation (install/upgrade/rollback) is in progress")
)

// ValidName is a regular expression for resource names.
//
// DEPRECATED: This will be removed in Helm 4, and is no longer used here. See
// pkg/lint/rules.validateMetadataNameFunc for the replacement.
//
// According to the Kubernetes help text, the regular expression it uses is:
//
//	[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
//
// This follows the above regular expression (but requires a full string match, not partial).
//
// The Kubernetes documentation is here, though it is not entirely correct:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
var ValidName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

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

	// HookOutputFunc called with container name and returns and expects writer that will receive the log output.
	HookOutputFunc func(namespace, pod, container string) io.Writer
}

// renderResources renders the templates in a chart
//
// TODO: This function is badly in need of a refactor.
// TODO: As part of the refactor the duplicate code in cmd/helm/template.go should be removed
//
//	This code has to do with writing files to disk.
func (cfg *Configuration) renderResources(ch *chart.Chart, values chartutil.Values, releaseName, outputDir string, subNotes, useReleaseName, includeCrds bool, pr postrender.PostRenderer, interactWithRemote, enableDNS, hideSecret bool) ([]*release.Hook, *bytes.Buffer, string, error) {
	hs := []*release.Hook{}
	b := bytes.NewBuffer(nil)

	caps, err := cfg.getCapabilities()
	if err != nil {
		return hs, b, "", err
	}

	if ch.Metadata.KubeVersion != "" {
		if !chartutil.IsCompatibleRange(ch.Metadata.KubeVersion, caps.KubeVersion.String()) {
			return hs, b, "", errors.Errorf("chart requires kubeVersion: %s which is incompatible with Kubernetes %s", ch.Metadata.KubeVersion, caps.KubeVersion.String())
		}
	}

	var files map[string]string
	var err2 error

	// A `helm template` should not talk to the remote cluster. However, commands with the flag
	// `--dry-run` with the value of `false`, `none`, or `server` should try to interact with the cluster.
	// It may break in interesting and exotic ways because other data (e.g. discovery) is mocked.
	if interactWithRemote && cfg.RESTClientGetter != nil {
		restConfig, err := cfg.RESTClientGetter.ToRESTConfig()
		if err != nil {
			return hs, b, "", err
		}
		e := engine.New(restConfig)
		e.EnableDNS = enableDNS
		files, err2 = e.Render(ch, values)
	} else {
		var e engine.Engine
		e.EnableDNS = enableDNS
		files, err2 = e.Render(ch, values)
	}

	if err2 != nil {
		return hs, b, "", err2
	}

	// NOTES.txt gets rendered like all the other files, but because it's not a hook nor a resource,
	// pull it out of here into a separate file so that we can actually use the output of the rendered
	// text file. We have to spin through this map because the file contains path information, so we
	// look for terminating NOTES.txt. We also remove it from the files so that we don't have to skip
	// it in the sortHooks.
	var notesBuffer bytes.Buffer
	for k, v := range files {
		if strings.HasSuffix(k, notesFileSuffix) {
			if subNotes || (k == path.Join(ch.Name(), "templates", notesFileSuffix)) {
				// If buffer contains data, add newline before adding more
				if notesBuffer.Len() > 0 {
					notesBuffer.WriteString("\n")
				}
				notesBuffer.WriteString(v)
			}
			delete(files, k)
		}
	}
	notes := notesBuffer.String()

	// Sort hooks, manifests, and partials. Only hooks and manifests are returned,
	// as partials are not used after renderer.Render. Empty manifests are also
	// removed here.
	hs, manifests, err := releaseutil.SortManifests(files, nil, releaseutil.InstallOrder)
	if err != nil {
		// By catching parse errors here, we can prevent bogus releases from going
		// to Kubernetes.
		//
		// We return the files as a big blob of data to help the user debug parser
		// errors.
		for name, content := range files {
			if strings.TrimSpace(content) == "" {
				continue
			}
			fmt.Fprintf(b, "---\n# Source: %s\n%s\n", name, content)
		}
		return hs, b, "", err
	}

	// Aggregate all valid manifests into one big doc.
	fileWritten := make(map[string]bool)

	if includeCrds {
		for _, crd := range ch.CRDObjects() {
			if outputDir == "" {
				fmt.Fprintf(b, "---\n# Source: %s\n%s\n", crd.Filename, string(crd.File.Data[:]))
			} else {
				err = writeToFile(outputDir, crd.Filename, string(crd.File.Data[:]), fileWritten[crd.Filename])
				if err != nil {
					return hs, b, "", err
				}
				fileWritten[crd.Filename] = true
			}
		}
	}

	for _, m := range manifests {
		if outputDir == "" {
			if hideSecret && m.Head.Kind == "Secret" && m.Head.Version == "v1" {
				fmt.Fprintf(b, "---\n# Source: %s\n# HIDDEN: The Secret output has been suppressed\n", m.Name)
			} else {
				fmt.Fprintf(b, "---\n# Source: %s\n%s\n", m.Name, m.Content)
			}
		} else {
			newDir := outputDir
			if useReleaseName {
				newDir = filepath.Join(outputDir, releaseName)
			}
			// NOTE: We do not have to worry about the post-renderer because
			// output dir is only used by `helm template`. In the next major
			// release, we should move this logic to template only as it is not
			// used by install or upgrade
			err = writeToFile(newDir, m.Name, m.Content, fileWritten[m.Name])
			if err != nil {
				return hs, b, "", err
			}
			fileWritten[m.Name] = true
		}
	}

	if pr != nil {
		b, err = pr.Run(b)
		if err != nil {
			return hs, b, notes, errors.Wrap(err, "error while running post render on files")
		}
	}

	return hs, b, notes, nil
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
func (cfg *Configuration) getCapabilities() (*chartutil.Capabilities, error) {
	if cfg.Capabilities != nil {
		return cfg.Capabilities, nil
	}
	dc, err := cfg.RESTClientGetter.ToDiscoveryClient()
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
			cfg.Log("WARNING: The Kubernetes server has an orphaned API service. Server reports: %s", err)
			cfg.Log("WARNING: To fix this, kubectl delete apiservice <service-name>")
		} else {
			return nil, errors.Wrap(err, "could not get apiVersions from Kubernetes")
		}
	}

	cfg.Capabilities = &chartutil.Capabilities{
		APIVersions: apiVersions,
		KubeVersion: chartutil.KubeVersion{
			Version: kubeVersion.GitVersion,
			Major:   kubeVersion.Major,
			Minor:   kubeVersion.Minor,
		},
		HelmVersion: chartutil.DefaultCapabilities.HelmVersion,
	}
	return cfg.Capabilities, nil
}

// KubernetesClientSet creates a new kubernetes ClientSet based on the configuration
func (cfg *Configuration) KubernetesClientSet() (kubernetes.Interface, error) {
	conf, err := cfg.RESTClientGetter.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "unable to generate config for kubernetes client")
	}

	return kubernetes.NewForConfig(conf)
}

// Now generates a timestamp
//
// If the configuration has a Timestamper on it, that will be used.
// Otherwise, this will use time.Now().
func (cfg *Configuration) Now() time.Time {
	return Timestamper()
}

func (cfg *Configuration) releaseContent(name string, version int) (*release.Release, error) {
	if err := chartutil.ValidateReleaseName(name); err != nil {
		return nil, errors.Errorf("releaseContent: Release name is invalid: %s", name)
	}

	if version <= 0 {
		return cfg.Releases.Last(name)
	}

	return cfg.Releases.Get(name, version)
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
	var versions []string

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
func (cfg *Configuration) recordRelease(r *release.Release) {
	if err := cfg.Releases.Update(r); err != nil {
		cfg.Log("warning: Failed to update release %s: %s", r.Name, err)
	}
}

// Init initializes the action configuration
func (cfg *Configuration) Init(getter genericclioptions.RESTClientGetter, namespace, helmDriver string, log DebugLog) error {
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
		if cfg.Releases != nil {
			if mem, ok := cfg.Releases.Driver.(*driver.Memory); ok {
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
			return errors.Wrap(err, "unable to instantiate SQL driver")
		}
		store = storage.Init(d)
	default:
		return errors.Errorf("unknown driver %q", helmDriver)
	}

	cfg.RESTClientGetter = getter
	cfg.KubeClient = kc
	cfg.Releases = store
	cfg.Log = log
	cfg.HookOutputFunc = func(_, _, _ string) io.Writer { return io.Discard }

	return nil
}

// SetHookOutputFunc sets the HookOutputFunc on the Configuration.
func (cfg *Configuration) SetHookOutputFunc(hookOutputFunc func(_, _, _ string) io.Writer) {
	cfg.HookOutputFunc = hookOutputFunc
}
