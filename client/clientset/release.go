package clientset

import (
	aci "k8s.io/helm/api"
	"k8s.io/kubernetes/pkg/api"
	rest "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/watch"
)

type ReleaseNamespacer interface {
	Release(namespace string) ReleaseInterface
}

type ReleaseInterface interface {
	List(opts api.ListOptions) (*aci.ReleaseList, error)
	Get(name string) (*aci.Release, error)
	Create(release *aci.Release) (*aci.Release, error)
	Update(release *aci.Release) (*aci.Release, error)
	Delete(name string) error
	Watch(opts api.ListOptions) (watch.Interface, error)
	UpdateStatus(release *aci.Release) (*aci.Release, error)
}

type ReleaseImpl struct {
	r  rest.Interface
	ns string
}

func newRelease(c *ExtensionsClient, namespace string) *ReleaseImpl {
	return &ReleaseImpl{c.restClient, namespace}
}

func (c *ReleaseImpl) List(opts api.ListOptions) (result *aci.ReleaseList, err error) {
	result = &aci.ReleaseList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("releases").
		VersionedParams(&opts, ExtendedCodec).
		Do().
		Into(result)
	return
}

func (c *ReleaseImpl) Get(name string) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("releases").
		Name(name).
		Do().
		Into(result)
	return
}

func (c *ReleaseImpl) Create(release *aci.Release) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.r.Post().
		Namespace(c.ns).
		Resource("releases").
		Body(release).
		Do().
		Into(result)
	return
}

func (c *ReleaseImpl) Update(release *aci.Release) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.r.Put().
		Namespace(c.ns).
		Resource("releases").
		Name(release.Name).
		Body(release).
		Do().
		Into(result)
	return
}

func (c *ReleaseImpl) Delete(name string) (err error) {
	return c.r.Delete().
		Namespace(c.ns).
		Resource("releases").
		Name(name).
		Do().
		Error()
}

func (c *ReleaseImpl) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("releases").
		VersionedParams(&opts, ExtendedCodec).
		Watch()
}

func (c *ReleaseImpl) UpdateStatus(release *aci.Release) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.r.Put().
		Namespace(c.ns).
		Resource("releases").
		Name(release.Name).
		SubResource("status").
		Body(release).
		Do().
		Into(result)
	return
}
