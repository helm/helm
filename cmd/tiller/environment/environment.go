/*Package environment describes the operating environment for Tiller.

Tiller's environment encapsulates all of the service dependencies Tiller has.
These dependencies are expressed as interfaces so that alternate implementations
(mocks, etc.) can be easily generated.
*/
package environment

import (
	"io"

	"github.com/kubernetes/helm/pkg/engine"
	"github.com/kubernetes/helm/pkg/kube"
	"github.com/kubernetes/helm/pkg/proto/hapi/chart"
	"github.com/kubernetes/helm/pkg/proto/hapi/release"
	"github.com/kubernetes/helm/pkg/storage"
)

// GoTplEngine is the name of the Go template engine, as registered in the EngineYard.
const GoTplEngine = "gotpl"

// DefaultNamespace is the default namespace for Tiller.
const DefaultNamespace = "helm"

// DefaultEngine points to the engine that the EngineYard should treat as the
// default. A chart that does not specify an engine may be run through the
// default engine.
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
	// Render renders a chart.
	//
	// It receives a chart, a config, and a map of overrides to the config.
	// Overrides are assumed to be passed from the system, not the user.
	Render(*chart.Chart, *chart.Config, map[string]interface{}) (map[string]string, error)
}

// ReleaseStorage represents a storage engine for a Release.
//
// Release storage must be concurrency safe.
type ReleaseStorage interface {

	// Create stores a release in the storage.
	//
	// If a release with the same name exists, this returns an error.
	//
	// It may return other errors in cases where it cannot write to storage.
	Create(*release.Release) error
	// Read takes a name and returns a release that has that name.
	//
	// It will only return releases that are not deleted and not superseded.
	//
	// It will return an error if no relevant release can be found, or if storage
	// is not properly functioning.
	Read(name string) (*release.Release, error)

	// Update looks for a release with the same name and updates it with the
	// present release contents.
	//
	// For immutable storage backends, this may result in a new release record
	// being created, and the previous release being marked as superseded.
	//
	// It will return an error if a previous release is not found. It may also
	// return an error if the storage backend encounters an error.
	Update(*release.Release) error

	// Delete marks a Release as deleted.
	//
	// It returns the deleted record. If the record is not found or if the
	// underlying storage encounters an error, this will return an error.
	Delete(name string) (*release.Release, error)

	// List lists all active (non-deleted, non-superseded) releases.
	//
	// To get deleted or superseded releases, use Query.
	List() ([]*release.Release, error)

	// Query takes a map of labels and returns any releases that match.
	//
	// Query will search all releases, including deleted and superseded ones.
	// The provided map will be used to filter results.
	Query(map[string]string) ([]*release.Release, error)

	// History takes a release name and returns the history of releases.
	History(name string) ([]*release.Release, error)
}

// KubeClient represents a client capable of communicating with the Kubernetes API.
//
// A KubeClient must be concurrency safe.
type KubeClient interface {
	// Create creates one or more resources.
	//
	// namespace must contain a valid existing namespace.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Create(namespace string, reader io.Reader) error

	// Delete destroys one or more resources.
	//
	// namespace must contain a valid existing namespace.
	//
	// reader must contain a YAML stream (one or more YAML documents separated
	// by "\n---\n").
	Delete(namespace string, reader io.Reader) error
}

// PrintingKubeClient implements KubeClient, but simply prints the reader to
// the given output.
type PrintingKubeClient struct {
	Out io.Writer
}

// Create prints the values of what would be created with a real KubeClient.
func (p *PrintingKubeClient) Create(ns string, r io.Reader) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// Delete implements KubeClient delete.
//
// It only prints out the content to be deleted.
func (p *PrintingKubeClient) Delete(ns string, r io.Reader) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// Environment provides the context for executing a client request.
//
// All services in a context are concurrency safe.
type Environment struct {
	// The default namespace
	Namespace string
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
	var ey EngineYard = map[string]Engine{
		// Currently, the only template engine we support is the GoTpl one. But
		// we can easily add some here.
		GoTplEngine: e,
	}
	return &Environment{
		Namespace:  DefaultNamespace,
		EngineYard: ey,
		Releases:   storage.NewMemory(),
		KubeClient: kube.New(nil), //&PrintingKubeClient{Out: os.Stdout},
	}
}
