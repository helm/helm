package api

import (
	schema "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
	versionedwatch "k8s.io/kubernetes/pkg/watch/versioned"
)

// SchemeGroupVersion is group version used to register these objects
var V1alpha1SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

var (
	V1alpha1SchemeBuilder = runtime.NewSchemeBuilder(v1addKnownTypes)
	V1betaAddToScheme     = V1alpha1SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func v1addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(V1alpha1SchemeGroupVersion,
		&Release{},
		&ReleaseList{},

		&v1.ListOptions{},
		&v1.DeleteOptions{},
	)
	versionedwatch.AddToGroupVersion(scheme, V1alpha1SchemeGroupVersion)
	return nil
}
