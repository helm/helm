package client

import aci "k8s.io/helm/api"

// The ReleaseExpansion interface allows manually adding extra methods to the ReleaseInterface.
type ReleaseExpansion interface {
	Dryrun(*aci.Release) (*aci.Release, error)
}

// Dryrun applied the provided ReleaseDryrun to the named release in the current namespace.
func (c *releases) Dryrun(release *aci.Release) (result *aci.Release, err error) {
	result = &aci.Release{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("releases").
		Name(release.Name).
		SubResource("dryrun").
		Body(release).
		Do().
		Into(result)
	return
}
