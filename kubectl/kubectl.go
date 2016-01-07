package kubectl

// Path is the path of the kubectl binary
var Path = "kubectl"

// Runner is an interface to wrap kubectl convenience methods
type Runner interface {
	// ClusterInfo returns Kubernetes cluster info
	ClusterInfo() ([]byte, error)
	// Create uploads a chart to Kubernetes
	Create([]byte, string) ([]byte, error)
	// Delete removes a chart from Kubernetes.
	Delete(string, string, string) ([]byte, error)
	// Get returns Kubernetes resources
	Get([]byte, string) ([]byte, error)

	// GetByKind gets an entry by kind, name, and namespace.
	//
	// If name is omitted, all entries of that kind are returned.
	//
	// If NS is omitted, the default NS is assumed.
	GetByKind(kind, name, ns string) (string, error)
}

// RealRunner implements Runner to execute kubectl commands
type RealRunner struct{}

// PrintRunner implements Runner to return a []byte of the command to be executed
type PrintRunner struct{}

// Client stores the instance of Runner
var Client Runner = RealRunner{}
