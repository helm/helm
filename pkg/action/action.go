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

	"helm.sh/helm/v3/internal/experimental/registry"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/postrender"
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
	// errInvalidName indicates that an invalid release name was provided
	errInvalidName = errors.New("invalid release name, must match regex ^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$ and the length must not longer than 53")
	// errPending indicates that another instance of Helm is already applying an operation on a release.
	errPending = errors.New("another operation (install/upgrade/rollback) is in progress")
)

// ValidName is a regular expression for resource names.
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
}

// renderedResources is an internal representation of a rendered set of resources
type renderedResources struct {
	hooks     []*renderedDocument
	resources []*renderedDocument
	notes     string
	crds      []*renderedDocument
}

// writeDirectory creates a directory and writes the rendered resources to that directory
//
// This will return an error if the directory already exists, if it can't be created, or
// if any file fails to write.
func (r *renderedResources) writeDirectory(outdir string, flags renderFlags) error {
	mode := os.FileMode(0755)
	if _, err := os.Stat(outdir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unexpected error with directory %q: %s", outdir, err)
	}

	if err := os.MkdirAll(outdir, mode); err != nil {
		return err
	}

	all := []*renderedDocument{}

	if flags.includeCrds {
		all = append(all, r.crds...)
	}

	if flags.includeHooks {
		all = append(all, r.hooks...)
	}

	all = append(all, r.resources...)

	// Write each file
	for _, doc := range all {
		// Write to disk
		filename := filepath.Join(outdir, doc.name)
		// Because doc.name pay have path components, we need to make sure all of the
		// directories are created before we attempt to write a file.
		destdir := filepath.Dir(filename)
		if err := os.MkdirAll(destdir, 0755); err != nil {
			errors.Wrapf(err, "could not create dir %s", destdir)
		}

		fh, err := os.OpenFile(filename, os.O_APPEND|os.O_RDWR|os.O_CREATE, mode)
		if err != nil {
			return errors.Wrapf(err, "could not create/append to file %s", filename)
		}
		_, err = fh.Write(doc.asBuffer().Bytes())
		fh.Close()
		if err != nil {
			return errors.Wrapf(err, "could not write to file %s", filename)
		}
	}
	return nil
}

// Get the resources as a single manifest
func (r *renderedResources) manifest() string {
	b := bytes.Buffer{}
	for _, item := range r.resources {
		//b.WriteString("\n---\n# Source: ")
		//b.WriteString(item.name)
		//b.WriteRune('\n')
		b.WriteString("---\n")
		b.WriteString(item.content)
	}
	return b.String()
}

// toBuffer gets the resources as a bytes.Buffer
//
// Depending on flags, this may return hooks and CRDs
func (r *renderedResources) toBuffer(flags renderFlags) *bytes.Buffer {
	b := bytes.Buffer{}
	writeItem := func(item *renderedDocument) {
		//b.WriteString("---\n# Source: ")
		//b.WriteString(item.name)
		//b.WriteRune('\n')
		b.WriteString("---\n")
		b.WriteString(item.content)
	}

	// CRDs are always first
	if flags.includeCrds {
		for _, item := range r.crds {
			writeItem(item)
		}
	}

	// Regular files
	for _, item := range r.resources {
		writeItem(item)
	}

	// Hooks are last
	if flags.includeHooks {
		for _, item := range r.hooks {
			writeItem(item)
		}
	}
	return &b
}

// releaseHooks returns the hooks formatted as release.Hook objects
// This might be unnecessary if postRender is done well
func (r *renderedResources) releaseHooks(c *Configuration) ([]*release.Hook, error) {
	files := map[string]string{}
	for _, h := range r.hooks {
		files[h.name] = h.content
	}
	caps, err := c.getCapabilities()
	if err != nil {
		return nil, err
	}
	hooks, _, err := releaseutil.SortManifests(files, caps.APIVersions, releaseutil.InstallOrder)
	return hooks, err
}

// postRender calls the post-renderer on resources, then tries to reset the object
//
// post-render collapses all of the resources into one long byte array. This is a problem
// because the file objects are lost, and there is no way to pick out the resulting
// resources and re-assign them to file names.
func (r *renderedResources) postRender(pr postrender.PostRenderer, flags renderFlags) (*bytes.Buffer, error) {
	b := r.toBuffer(flags)
	var err error
	b, err = pr.Run(b)
	if err != nil {
		return b, errors.Wrap(err, "error while running post render on files")
	}

	// We need to get hooks back out of this, which is an epic hack because they
	// were all just shoved into the same I/O cycle with the rest of the manifests.
	// The SortManifests function is our best tool for finding hooks, so we can use that.
	// But first we have to split all of the manifests again and give them fake
	// filenames because the filenames were lost. Or maybe we can just parse the manifests
	// and look at the labels. Not sure we need filenames.

	// Note that you SHOULD NOT try to add special CRD handling here. Any CRD that is
	// in the `crds/` directory SHOULD NOT be sent to the post-rendered. Once it is sent
	// to post-render, it is indistinguishable from templated files (including CRDs that
	// were declared that way). So if someone wants to add CRD support to postRender, those
	// CRDs must be put through the renderer as a _separate operation_, not as part of
	// the main post-render. Please do not try clever hacks around this system, because
	// we will not trust the post-renderer to correctly distinguish between CRDs that
	// should be pre-loaded and those that should be loaded with the rest of the chart.
	// In other words, attempting to use labels or annotations to distinguish CRDs is not
	// a good idea, because the post-renderer could delete or manipulate those, which can
	// have long-term implications for managing the installation.

	return b, nil
}

// renderedDocument represents a rendered piece of generic YAML content
type renderedDocument struct {
	name    string
	content string
}

// asBuffer converts the document to a Buffer containing the serialized document contents.
//
// This prepends the stream separator (---) to the buffer.
func (r *renderedDocument) asBuffer() *bytes.Buffer {
	buf := bytes.NewBufferString("---\n")
	buf.WriteString(r.content)
	return buf
}

// renderFlags is the flag set for rendering content
type renderFlags struct {
	dryRun       bool
	subNotes     bool
	includeCrds  bool
	includeHooks bool
}

const sourceComment = "# Source: %s\n%s\n"

func (c *Configuration) renderResources2(ch *chart.Chart, values chartutil.Values, flags renderFlags) (*renderedResources, error) {
	res := &renderedResources{}

	// Get the capabilities from k8s
	// TODO: If `helm template` is called, do we skip this?
	caps, err := c.getCapabilities()
	if err != nil {
		return res, err
	}

	// If chart is restricted to particular k8s version, verify a supported version
	if ch.Metadata.KubeVersion != "" {
		if !chartutil.IsCompatibleRange(ch.Metadata.KubeVersion, caps.KubeVersion.String()) {
			return res, errors.Errorf("chart requires kubeVersion: %s which is incompatible with Kubernetes %s", ch.Metadata.KubeVersion, caps.KubeVersion.String())
		}
	}

	var files map[string]string
	var err2 error

	// A `helm template` or `helm install --dry-run` should not talk to the remote cluster.
	// It will break in interesting and exotic ways because other data (e.g. discovery)
	// is mocked. It is not up to the template author to decide when the user wants to
	// connect to the cluster. So when the user says to dry run, respect the user's
	// wishes and do not connect to the cluster.
	if !flags.dryRun && c.RESTClientGetter != nil {
		rest, err := c.RESTClientGetter.ToRESTConfig()
		if err != nil {
			return res, err
		}
		files, err2 = engine.RenderWithClient(ch, values, rest)
	} else {
		files, err2 = engine.Render(ch, values)
	}

	if err2 != nil {
		return res, err2
	}

	// Copy the CRDs into the results
	for _, c := range ch.CRDObjects() {
		res.crds = append(res.crds, &renderedDocument{
			name:    c.Name, // Is this a bug? Shouldn't it be c.Filename?
			content: fmt.Sprintf(sourceComment, c.Name, c.File.Data[:]),
		})
	}

	// NOTES.txt gets rendered like all the other files, but because it's not a hook nor a resource,
	// pull it out of here into a separate file so that we can actually use the output of the rendered
	// text file. We have to spin through this map because the file contains path information, so we
	// look for terminating NOTES.txt. We also remove it from the files so that we don't have to skip
	// it in the sortHooks.
	var notesBuffer bytes.Buffer
	for k, v := range files {
		if strings.HasSuffix(k, notesFileSuffix) {
			if flags.subNotes || (k == path.Join(ch.Name(), "templates", notesFileSuffix)) {
				// If buffer contains data, add newline before adding more
				if notesBuffer.Len() > 0 {
					notesBuffer.WriteString("\n")
				}
				notesBuffer.WriteString(v)
			}
			delete(files, k)
		}
	}
	res.notes = notesBuffer.String()

	// Sort hooks, manifests, and partials. Only hooks and manifests are returned,
	// as partials are not used after renderer.Render. Empty manifests are also
	// removed here.
	hs, manifests, err := releaseutil.SortManifests(files, caps.APIVersions, releaseutil.InstallOrder)
	if err != nil {
		// By catching parse errors here, we can prevent bogus releases from going
		// to Kubernetes.
		//
		// We return the files to help the user debug parser errors.
		//
		// TODO: Why not use a custom error type to do this?
		for name, content := range files {
			if strings.TrimSpace(content) == "" {
				continue
			}
			// Otherwise, insert this into the results
			doc := renderedDocument{
				name:    name,
				content: fmt.Sprintf(sourceComment, name, content),
			}
			res.resources = append(res.resources, &doc)
		}
		return res, err
	}

	// Copy the hooks into the result
	//res.hooks = hs
	for _, h := range hs {
		res.hooks = append(res.hooks, &renderedDocument{
			name:    h.Path,
			content: fmt.Sprintf(sourceComment, h.Path, h.Manifest),
		})
	}

	// Copy the manifests
	for _, m := range manifests {
		res.resources = append(res.resources, &renderedDocument{
			name:    m.Name,
			content: fmt.Sprintf(sourceComment, m.Name, m.Content),
		})
	}

	return res, nil
}

// renderResources renders the templates in a chart
//
// TODO: This function is badly in need of a refactor.
/*
func (c *Configuration) renderResources(ch *chart.Chart, values chartutil.Values, releaseName, outputDir string, subNotes, useReleaseName, includeCrds bool, disableHooks bool, pr postrender.PostRenderer, dryRun bool) ([]*release.Hook, *bytes.Buffer, string, error) {
	hs := []*release.Hook{}
	b := bytes.NewBuffer(nil)

	caps, err := c.getCapabilities()
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

	// A `helm template` or `helm install --dry-run` should not talk to the remote cluster.
	// It will break in interesting and exotic ways because other data (e.g. discovery)
	// is mocked. It is not up to the template author to decide when the user wants to
	// connect to the cluster. So when the user says to dry run, respect the user's
	// wishes and do not connect to the cluster.
	if !dryRun && c.RESTClientGetter != nil {
		rest, err := c.RESTClientGetter.ToRESTConfig()
		if err != nil {
			return hs, b, "", err
		}
		files, err2 = engine.RenderWithClient(ch, values, rest)
	} else {
		files, err2 = engine.Render(ch, values)
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
	hs, manifests, err := releaseutil.SortManifests(files, caps.APIVersions, releaseutil.InstallOrder)
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
				fmt.Fprintf(b, "---\n# Source: %s\n%s\n", crd.Name, string(crd.File.Data[:]))
			} else {
				err = writeToFile(outputDir, crd.Filename, string(crd.File.Data[:]), fileWritten[crd.Name])
				if err != nil {
					return hs, b, "", err
				}
				fileWritten[crd.Name] = true
			}
		}
	}

	for _, m := range manifests {
		if outputDir == "" {
			fmt.Fprintf(b, "---\n# Source: %s\n%s\n", m.Name, m.Content)
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

	if !disableHooks && len(hs) > 0 {
		for _, h := range hs {
			if outputDir == "" {
				fmt.Fprintf(b, "---\n# Source: %s\n%s\n", h.Path, h.Manifest)
			} else {
				newDir := outputDir
				if useReleaseName {
					newDir = filepath.Join(outputDir, releaseName)
				}
				err = writeToFile(newDir, h.Path, h.Manifest, fileWritten[h.Path])
				if err != nil {
					return hs, b, "", err
				}
				fileWritten[h.Path] = true
			}
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
*/

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
