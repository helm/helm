# Proposal: Support managing releases for external clusters in Tiller

## User Story

Company X runs a SAAS platform whose customers can deploy Kubernetes clusters on various cloud providers using a simple web interface. Company Y is a customer of company X. Y uses Kubernetes to run its microservices apis. Y has 4 clusters:

* A non-federated qa cluster in US.

* A federated prod cluster with clusters in US, EU and Asia regions.

Now company X wants to build an App Dashboard web ui from where its customers like Y can manage their application deployment process and maintain history of releases across various clusters. App Dashboard should support these features:

* **S1:** Provide a unified view of an application across various environments (qa, prod etc).

* **S2:** Release apps to a cluster from web ui

* **S3:** See past history of releases even for clusters that don’t exist any more. This is required for record keeping purposes.

* **S4:** Can release pre-built apps or deploy internal apps.

* **S5:** Avoid privilege escalation issues via release process. Maintain [authentication](http://kubernetes.io/docs/admin/authentication/) and [authorization](http://kubernetes.io/docs/admin/authorization/) applied to clusters via authn/authz plugins (eg RBAC, webhook, etc.). Support release processes in a multi-tenant cluster. A common model of multi-tenant cluster is different groups of users are restricted to a different Kubernetes namespace.

## Limitations of Tiller Server in Helm 2.x

* Accessing Tiller server gRPC api from a web page is difficult for a number of reason:

    * Tiller server uses gRPC api that does not work with AJAX. This can be solved with using [gRPC Gateway](https://github.com/grpc-ecosystem/grpc-gateway).

    * Tiller server does not have any built-in authentication mechanism. So, if this is exposed via gRPC gateway, further work is needed to secure access to Tiller server.

* Currently Tiller server stores data inside configmaps. This means release history is available as long as Kubernetes cluster is alive.

* Kubernetes clusters support authorization via RBAC, Webhook methods. Tiller server uses in cluster config to connect to Kubernetes api server and ignores authorization configured in a Kubernetes cluster.

* Tiller server manages release for a single Kubernetes cluster. This means a separate Tiller server is needed for each cluster.

## Proposed Design

In light of the above user story, I propose that Tiller server be reimplemented using Kubernetes 3rd party objects. In this approach, Kubernetes api server will provide the HTTP api that can be used by Helm cli or from a web page via AJX calls. The release logic can be converted to a controller pod that watches for releases and perform the actual operation of releasing a chart. The pros of this approach are:

* Since this model uses the existing Kubernetes api server, this simplifies the implementation of Tiller server. This provides an HTTP/1.1 api that can be accessed from Helm cli and a web page via AJAX.

* Release objects can take advantage of the Authentication and Authorization features of Kubernetes api server.

* This design can handle multi-tenant Kubernetes cluster usage models.

* I have not used a federated Kubernetes cluster yet. But from reading the docs, my understanding is that Tiller controller needs to use the Federated api server and templates should use federated versions of Kubernetes resources.

The cons of this design are:

* Tiller controller only works on the cluster where it is installed. But this does not add any extra complexity to any Web Dashboard that can already handle multiple Kubernetes clusters.

* Since data about 3rd party objects are stored in the etcd server used by Kubernetes api server, past history of releases will not be accessible once a cluster is deleted. A generic solution for this be support webhooks with Tiller controller. Tiller controller process can call send a POST request with release status data after performing various actions on a release.

* A Release object can deploy resources only in its own namespace. This is necessary to support multi-tenant clusters. Today it is possible to deploy objects in any namespace of a cluster. So, this will reduce functionality that is available currently.

* This will be backwards incompatible with Helm cli. So, this will require a major release. It will require additional work to migrate existing release data into the 3rd party objects.

Here is a proposed design for the 3rd party objects. This is based on existing Helm protos and Kubernetes Deployment object and interface. I have skipped the internals for objects that will remain unchanged. I can add more details if the general idea of this proposal is acceptable.

```go

// Chart represents a named chart that is installed in a Release.
type Chart struct {
	Name string `json:"name"`
	// The ChartSource represents the location and type of a chart to install.
	// This is modelled like Volume in Pods, which allows specifying a chart
	// inline (like today) or pulling a chart object from a (potentially private) chart registry similar to pulling a Docker image.
	// +optional
	ChartSource `json:",inline,omitempty"`
}

type ChartSource struct {
    // Inline charts are what is done today with Helm cli. Release request
    // contains the chart definition in the release spec, sent by Helm cli.
	Inline hapi.chart.Chart `json:"inline,omitempty"`

    // This can be similar to Image in PodSpec, where a reference to a
    // extrnally hosted chart is provided. This can also include name of a
    // secret that is used to access a private chart registry.
	ChartRef *ChartRegistryRef `json:"inline,omitempty"`

    // Anyone can add support for their custom chart registry by extending
    // Tiller controller. like, CNR reg, etc.
}

type ReleaseSpec struct {
	Chart Chart `protobuf:"bytes,1,opt,name=chart" json:"chart,omitempty"`
	// Values is a string containing (unparsed) YAML values.
	Values hapi_chart.Config `protobuf:"bytes,2,opt,name=values" json:"values,omitempty"`
	// DryRun, if true, will run through the release logic, but neither create
	// a release object nor deploy to Kubernetes. The release object returned
	// in the response will be fake.
	DryRun bool `protobuf:"varint,3,opt,name=dry_run,json=dryRun" json:"dry_run,omitempty"`
	// Name is the candidate release name. This must be unique to the
	// namespace, otherwise the server will return an error. If it is not
	// supplied, the server will autogenerate one.
	Name string `protobuf:"bytes,4,opt,name=name" json:"name,omitempty"`
	// DisableHooks causes the server to skip running any hooks for the install.
	DisableHooks bool `protobuf:"varint,5,opt,name=disable_hooks,json=disableHooks" json:"disable_hooks,omitempty"`
}

type ReleaseStatus struct {
	// same as ReleaseStatusResponse
}

// Release is a represents the concept Release.
type Release struct {
	unversioned.TypeMeta `json:",inline"`
	// +optional
	api.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the Release.
	// +optional
	Spec ReleaseSpec `json:"spec,omitempty"`

	// Most recently observed status of the Release.
	// +optional
	Status ReleaseStatus `json:"status,omitempty"`
}

// DeploymentInterface has methods to work with Deployment resources.
type ReleaseInterface interface {
	Create(*extensions.Release) (*extensions.Release, error)
	Update(*extensions.Release) (*extensions.Release, error)
	UpdateStatus(*extensions.Release) (*extensions.Release, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*extensions.Release, error)
	List(opts api.ListOptions) (*extensions.ReleaseList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *extensions.Release, err error)
	ReleaseExpansion
}

// The ReleaseExpansion interface allows manually adding extra methods to the ReleaseInterface.
type ReleaseExpansion interface {
	Rollback(*extensions.ReleaseRollback) error

    // These following methods are similar to their duals but they allow creating/updating/deleting any secrets included in the chart.
    // Imagine a scenario where a user can create pods but can't create secrets. In a cluster like that, it will be possible to
    // overwwite secrets by including a secret with the same name in the chart. Here I propose that, the regular write methods will only
    // any object that is not a secret. In secrets are to be updated, the following set of method need to be called.
    // The following subresources can be secured using Kubernetes's authorization mechanisms.
	CreateWithSecrets(*extensions.Release) (*extensions.Release, error)
	UpdateWithSecrets(*extensions.Release) (*extensions.Release, error)
	DeleteWithSecrets(name string, options *api.DeleteOptions) error
	DeleteCollectionWithSecrets(options *api.DeleteOptions, listOptions api.ListOptions) error
	PatchWithSecrets(name string, pt api.PatchType, data []byte, subresources ...string) (result *extensions.Release, err error)
}

// Tiller controller should also create Event objects, so releases can be seen
// from `kubectl get events`.

//-----------------------------------------------------------------------------
// ReleaseVersion replaces the version wise configmaps used by Tiller 2.0

type ReleaseVersionSpec struct {
	// Chart is the protobuf representation of a chart.
	Chart *Chart `protobuf:"bytes,1,opt,name=chart" json:"chart,omitempty"`
	// Values is a string containing (unparsed) YAML values.
	Values *hapi_chart.Config `protobuf:"bytes,2,opt,name=values" json:"values,omitempty"`
	// Name is the candidate release name. This must be unique to the
	// namespace, otherwise the server will return an error. If it is not
	// supplied, the server will autogenerate one.
	Name string `protobuf:"bytes,4,opt,name=name" json:"name,omitempty"`
	// DisableHooks causes the server to skip running any hooks for the install.
	DisableHooks bool `protobuf:"varint,5,opt,name=disable_hooks,json=disableHooks" json:"disable_hooks,omitempty"`

	// In current design, the rendered objects are stored in gzip format in
	// the configmap. Here I propose that resources rendered are just stored
	// as references to actual objects.This can easily save some disk space.
	Resources []kube_api.ObjectReference `json:"resources,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,2,rep,name=resources"`
}

type ReleaseVersionStatus struct {
	// same as ReleaseStatusResponse
}

// ReleaseVersion replaces the Version wise configmaps used by Tiller 2.0
type ReleaseVersion struct {
	unversioned.TypeMeta `json:",inline"`
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a pod.
	// +optional
	Spec ReleaseVersionSpec `json:"spec,omitempty"`

	// Status represents the current information about a pod. This data may not be up
	// to date.
	// +optional
	Status ReleaseVersionStatus `json:"status,omitempty"`
}

// ReleaseVersion has methods to work with ReleaseVersion resources.
type ReleaseVersionInterface interface {
	Create(*extensions.ReleaseVersion) (*extensions.ReleaseVersion, error)
	Update(*extensions.ReleaseVersion) (*extensions.ReleaseVersion, error)
	UpdateStatus(*extensions.ReleaseVersion) (*extensions.ReleaseVersion, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*extensions.ReleaseVersion, error)
	List(opts api.ListOptions) (*extensions.ReleaseVersionList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
	Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *extensions.ReleaseVersion, err error)
}
```


## Alternative Design

An alternative design I have considered will require the following 2 changes to Tiller server 2.x implementation:

1. Include kube config with each request to Tiller server. This can be included as a field in the request protos. Helm cli can read this data from local Kube config. Web pages can pass bearer tokens as authentication in AJAX calls.

2. Include Storage Driver implementation that stores data in conventional databases like Postgres, etc. It should be very easy for implement this using existing Driver interface.

The main issue with this design is that it does not solve the issue of authenticating requests to Tiller server. This is can be solved using Authorization header for requests sent to Tiller server. This will essentially require re-implementing authentication strategies similar to Kubernetes api server so that cluster admins can use the authN strategy (OIDC tokens, Webhook, etc) for Kubernetes and Tiller server. This design will also require implementing authorization strategy to be used in a production grade setup.

This design has the advantage that a single Tiller server can handle many Kubernetes clusters.
