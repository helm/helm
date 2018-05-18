package events

import (
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/hapi/chart"
	//"k8s.io/helm/pkg/hapi/release"
)

type Context struct {

	// ReleaseName is the name of the release.
	ReleaseName string

	// Revision is the release revision. This is a ULID or empty if no release
	// has been stored.
	Revision string

	// Chart is the chart.
	Chart *chart.Metadata

	// Values is the override values (not the default values)
	Values chartutil.Values

	Notes string

	// Manifests is the manifests that Kubernetes will install.
	// Assume this is filename, content for now
	//Manifests map[string][]byte
	Manifests []string

	Hooks []string

	// Release is the release object
	Release chartutil.ReleaseOptions

	// Capabilities are passed by reference to avoid modifications bubbling up.
	Capabilities chartutil.Capabilities

	Files chartutil.Files
}
