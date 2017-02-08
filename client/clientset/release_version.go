package client

import (
	aci "k8s.io/helm/tillerc/api"
	"k8s.io/kubernetes/pkg/api"
	rest "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/watch"
)

type ReleaseVersionNamespacer interface {
	ReleaseVersion(namespace string) ReleaseVersionInterface
}

// ReleaseVersionInterface has methods to work with ReleaseVersion resources.
type ReleaseVersionInterface interface {
	Create(*aci.ReleaseVersion) (*aci.ReleaseVersion, error)
	Update(*aci.ReleaseVersion) (*aci.ReleaseVersion, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*aci.ReleaseVersion, error)
	List(opts api.ListOptions) (*aci.ReleaseVersionList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
}

// release-versions implements ReleaseVersionInterface
type releaseVersions struct {
	client rest.Interface
	ns     string
}

func newReleaseVersion(c *ExtensionsClient, namespace string) *releaseVersions {
	return &releaseVersions{c.restClient, namespace}
}

// newReleaseVersions returns a ReleaseVersions
func newReleaseVersions(c *ExtensionsClient, namespace string) *releaseVersions {
	return &releaseVersions{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a release and creates it.  Returns the server's representation of the release, and an error, if there is any.
func (c *releaseVersions) Create(version *aci.ReleaseVersion) (result *aci.ReleaseVersion, err error) {
	result = &aci.ReleaseVersion{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("releaseVersions").
		Body(version).
		Do().
		Into(result)
	return
}

// Delete takes name of the release and deletes it. Returns an error if one occurs.
func (c *releaseVersions) Delete(name string, options *api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("releaseVersions").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *releaseVersions) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("releaseVersions").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the release, and returns the corresponding release object, and an error if there is any.
func (c *releaseVersions) Get(name string) (result *aci.ReleaseVersion, err error) {
	result = &aci.ReleaseVersion{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("releaseVersions").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ReleaseVersions that match those selectors.
func (c *releaseVersions) List(opts api.ListOptions) (result *aci.ReleaseVersionList, err error) {
	result = &aci.ReleaseVersionList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("releaseVersions").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested release-versions.
func (c *releaseVersions) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("releaseVersions").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}

//updates release-version

func (c *releaseVersions) Update(version *aci.ReleaseVersion) (result *aci.ReleaseVersion, err error) {
	result = &aci.ReleaseVersion{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("releaseVersions").
		Name(version.Name).
		Body(version).
		Do().
		Into(result)
	return
}
