package router

import (
	"github.com/kubernetes/helm/cmd/manager/manager"
	"github.com/kubernetes/helm/pkg/common"
	helmhttp "github.com/kubernetes/helm/pkg/httputil"
)

// Config holds the global configuration parameters passed into the router.
//
// Config is used concurrently. Once a config is created, it should be treated
// as immutable.
type Config struct {
	// Address is the host and port (:8080)
	Address string
	// MaxTemplateLength is the maximum length of a template.
	MaxTemplateLength int64
	// ExpanderName is the DNS name of the expansion service.
	ExpanderName string
	// ExpanderURL is the expander service's URL.
	ExpanderURL string
	// DeployerName is the deployer's DNS name
	DeployerName string
	// DeployerURL is the deployer's URL
	DeployerURL string
	// CredentialFile is the file to the credentials.
	CredentialFile string
	// CredentialSecrets tells the service to use a secrets file instead.
	CredentialSecrets bool
	// MongoName is the DNS name of the mongo server.
	MongoName string
	// MongoPort is the port for the MongoDB protocol on the mongo server.
	// It is a string for historical reasons.
	MongoPort string
	// MongoAddress is the name and port.
	MongoAddress string
}

// Context contains dependencies that are passed to each handler function.
//
// Context carries typed information, often scoped to interfaces, so that the
// caller's contract with the service is known at compile time.
//
// Members of the context must be concurrency safe.
type Context struct {
	Config *Config
	// Manager is a helm/manager/manager.Manager
	Manager            manager.Manager
	Encoder            helmhttp.Encoder
	CredentialProvider common.CredentialProvider
}
