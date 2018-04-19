package release

import "k8s.io/helm/pkg/hapi/chart"

// Release describes a deployment of a chart, together with the chart
// and the variables used to deploy that chart.
type Release struct {
	// Name is the name of the release
	Name string `json:"name,omitempty"`
	// Info provides information about a release
	Info *Info `json:"info,omitempty"`
	// Chart is the chart that was released.
	Chart *chart.Chart `json:"chart,omitempty"`
	// Config is the set of extra Values added to the chart.
	// These values override the default values inside of the chart.
	Config *chart.Config `json:"config,omitempty"`
	// Manifest is the string representation of the rendered template.
	Manifest string `json:"manifest,omitempty"`
	// Hooks are all of the hooks declared for this release.
	Hooks []*Hook `json:"hooks,omitempty"`
	// Version is an int which represents the version of the release.
	Version int `json:"version,omitempty"`
	// Namespace is the kubernetes namespace of the release.
	Namespace string `json:"namespace,omitempty"`
}
