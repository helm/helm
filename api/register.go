package kube

import (
	"k8s.io/kubernetes/pkg/api"
	schema "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// GroupName is the group name use in this package
const GroupName = "helm.sh"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns back a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Release{},
		&ReleaseList{},

		&ReleaseVersion{},
		&ReleaseVersionList{},

		&api.ListOptions{},
		&api.DeleteOptions{},
	)
	return nil
}

func (obj *Release) GetObjectKind() schema.ObjectKind     { return &obj.TypeMeta }
func (obj *ReleaseList) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }

func (obj *ReleaseVersion) GetObjectKind() schema.ObjectKind     { return &obj.TypeMeta }
func (obj *ReleaseVersionList) GetObjectKind() schema.ObjectKind { return &obj.TypeMeta }
