package client

import (
	aci "k8s.io/helm/api"
	"k8s.io/kubernetes/pkg/api"
	rest "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/watch"
)

type ReleaseNamespacer interface {
	Release(namespace string) ReleaseInterface
}

// ReleaseInterface has methods to work with Release resources.
type ReleaseInterface interface {
	Create(*aci.Release) (*aci.Release, error)
	Update(*aci.Release) (*aci.Release, error)
	UpdateStatus(*aci.Release) (*aci.Release, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*aci.Release, error)
	List(opts api.ListOptions) (*aci.ReleaseList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
	ReleaseExpansion
}

// releases implements ReleaseInterface
type releases struct {
	client rest.Interface
	ns     string
}

func newRelease(c *ExtensionsClient, namespace string) *releases {
	return &releases{c.restClient, namespace}
}

// newReleases returns a Releases
func newReleases(c *ExtensionsClient, namespace string) *releases {
	return &releases{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Create takes the representation of a release and creates it.  Returns the server's representation of the release, and an error, if there is any.
func (c *releases) Create(release *aci.Release) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("releases").
		Body(release).
		Do().
		Into(result)
	return
}

// Update takes the representation of a release and updates it. Returns the server's representation of the release, and an error, if there is any.
func (c *releases) Update(release *aci.Release) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("releases").
		Name(release.Name).
		Body(release).
		Do().
		Into(result)
	return
}

func (c *releases) UpdateStatus(release *aci.Release) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("releases").
		Name(release.Name).
		SubResource("status").
		Body(release).
		Do().
		Into(result)
	return
}

// Delete takes name of the release and deletes it. Returns an error if one occurs.
func (c *releases) Delete(name string, options *api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("releases").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *releases) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("releases").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the release, and returns the corresponding release object, and an error if there is any.
func (c *releases) Get(name string) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("releases").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Releases that match those selectors.
func (c *releases) List(opts api.ListOptions) (result *aci.ReleaseList, err error) {
	result = &aci.ReleaseList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("releases").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested releases.
func (c *releases) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("releases").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}
