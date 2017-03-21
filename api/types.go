package api

import (
	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
)

// Release captures the state of a individual release and are immutable.
// Release replaces the version wise configmaps used by Tiller 2.0
type Release struct {
	unversioned.TypeMeta `json:",inline,omitempty"`
	api.ObjectMeta       `json:"metadata,omitempty"`
	Spec                 ReleaseSpec   `json:"spec,omitempty"`
	Status               ReleaseStatus `json:"status,omitempty"`
}

type ReleaseSpec struct {
	// Description is human-friendly "log entry" about this release.
	Description string `json:"Description,omitempty"`
	// Chart is the chart that was released.
	ChartMetadata *hapi_chart.Metadata `json:"chartMetadata,omitempty"`
	// Config is the set of extra Values added to the chart.
	// These values override the default values inside of the chart.
	Config *hapi_chart.Config `json:"config,omitempty"`
	// Version is an int32 which represents the version of the release.
	Version int32 `json:"version,omitempty"`

	// TODO(tamal): Store in proper namespace
	// Namespace is the kubernetes namespace of the release.
	// Namespace string `json:"namespace,omitempty"`

	// The ChartSource represents the location and type of a chart to install.
	// This is modelled like Volume in Pods, which allows specifying a chart
	// inline (like today) or pulling a chart object from a (potentially private)
	// chart registry similar to pulling a Docker image.
	Data ReleaseData `json:"data,omitempty"`
}

//-------------------------------------------------------------------------------------------
// Chart represents a chart that is installed in a Release.
// The ChartSource represents the location and type of a chart to install.
// This is modelled like Volume in Pods, which allows specifying a chart
// inline (like today) or pulling a chart object from a (potentially private) chart registry similar to pulling a Docker image.
// +optional
type ReleaseData struct {
	// Inline charts are what is done today with Helm cli. Release request
	// contains the chart definition in the release spec, sent by Helm cli.
	Inline string `json:"inline,omitempty"`
}

type ReleaseStatus struct {
	Code string `json:"code,omitempty"`
	// Cluster resources as kubectl would print them.
	Resources string `json:"resources,omitempty"`
	// Contains the rendered templates/NOTES.txt if available
	Notes         string           `json:"notes,omitempty"`
	FirstDeployed unversioned.Time `json:"first_deployed,omitempty"`
	LastDeployed  unversioned.Time `json:"last_deployed,omitempty"`
	// Deleted tracks when this object was deleted.
	Deleted unversioned.Time `json:"deleted,omitempty"`
}

type ReleaseList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []Release `json:"items,omitempty"`
}
