package environment

import (
	"github.com/deis/tiller/pkg/engine"
	"github.com/deis/tiller/pkg/hapi"
)

const GoTplEngine = "gotpl"

var DefaultEngine = GoTplEngine

// EngineYard maps engine names to engine implementations.
type EngineYard map[string]Engine

// Get retrieves a template engine by name.
//
// If no matching template engine is found, the second return value will
// be false.
func (y EngineYard) Get(k string) (Engine, bool) {
	e, ok := y[k]
	return e, ok
}

// Default returns the default template engine.
//
// The default is specified by DefaultEngine.
//
// If the default template engine cannot be found, this panics.
func (y EngineYard) Default() Engine {
	d, ok := y[DefaultEngine]
	if !ok {
		// This is a developer error!
		panic("Default template engine does not exist")
	}
	return d
}

// Engine represents a template engine that can render templates.
//
// For some engines, "rendering" includes both compiling and executing. (Other
// engines do not distinguish between phases.)
//
// The engine returns a map where the key is the named output entity (usually
// a file name) and the value is the rendered content of the template.
//
// An Engine must be capable of executing multiple concurrent requests, but
// without tainting one request's environment with data from another request.
type Engine interface {
	Render(*hapi.Chart, *hapi.Values) (map[string]string, error)
}

// ReleaseStorage represents a storage engine for a Release.
//
// Release storage must be concurrency safe.
type ReleaseStorage interface {
	// Get takes a name and returns the accompanying release.
	Get(key string) (*hapi.Release, error)
	// Set saves the release with the given name.
	Set(key string, val *hapi.Release) error
	// List lists all active (non-deleted, non-superseded) releases.
	//
	// To get deleted or superseded releases, use Query.
	List() ([]*hapi.Release, error)
	// Query takes a map of labels and returns any releases that match.
	Query(map[string]string) ([]*hapi.Release, error)
}

// KubeClient represents a client capable of communicating with the Kubernetes API.
//
// A KubeClient must be concurrency safe.
type KubeClient interface {
	// Install takes a map where the key is a "file name" (read: unique relational
	// id) and the value is a Kubernetes manifest containing one or more resource
	// definitions.
	//
	// TODO: Can these be in YAML or JSON, or must they be in one particular
	// format?
	Install(manifests map[string]string) error
}

// Environment provides the context for executing a client request.
//
// All services in a context are concurrency safe.
type Environment struct {
	// EngineYard provides access to the known template engines.
	EngineYard EngineYard
	// Releases stores records of releases.
	Releases ReleaseStorage
	// KubeClient is a Kubernetes API client.
	KubeClient KubeClient
}

// New returns an environment initialized with the defaults.
func New() *Environment {
	e := engine.New()
	var ey EngineYard = map[string]Engine{GoTplEngine: e}
	return &Environment{
		EngineYard: ey,
	}
}
