package action

import (
	"k8s.io/client-go/discovery"

	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/tiller/environment"
)

// Action describes a top-level Helm action.
//
// When implementing an action, the following guidelines should be observed:
//	- Constructors should take all REQUIRED fields
//	- Exported properties should hold all OPTIONAL fields
//
// When an error occurs, the result of 'Run()' should be targeted
// toward a user, but not assume a particular user interface (e.g. don't
// make reference to a command line flag).
type Action interface {
	Run() error
}

type Configuration struct {
	//engine    Engine
	discovery discovery.DiscoveryInterface

	// Releases stores records of releases.
	Releases *storage.Storage
	// KubeClient is a Kubernetes API client.
	KubeClient environment.KubeClient

	Log func(string, ...interface{})
}
