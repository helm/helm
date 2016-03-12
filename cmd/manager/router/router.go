/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*Package router is an HTTP router.

This router provides appropriate dependency injection/encapsulation for the
HTTP routing layer. This removes the requirement to set global variables for
resources like database handles.

This library does not replace the default HTTP mux because there is no need.
Instead, it implements an HTTP handler.

It then defines a handler function that is given a context as well as a
request and response.
*/
package router

import (
	"log"
	"net/http"

	"github.com/Masterminds/httputil"
	"github.com/kubernetes/deployment-manager/cmd/manager/manager"
	"github.com/kubernetes/deployment-manager/pkg/common"
	helmhttp "github.com/kubernetes/deployment-manager/pkg/httputil"
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
	// Manager is a deployment-manager/manager/manager.Manager
	Manager            manager.Manager
	Encoder            helmhttp.Encoder
	CredentialProvider common.CredentialProvider
}

// HandlerFunc responds to an individual HTTP request.
//
// Returned errors will be captured, logged, and returned as HTTP 500 errors.
type HandlerFunc func(w http.ResponseWriter, r *http.Request, c *Context) error

// Handler implements an http.Handler.
//
// This is the top level route handler.
type Handler struct {
	c        *Context
	resolver *httputil.Resolver
	routes   Routes
}

// NewHandler creates a new Handler.
//
// Routes cannot be modified after construction. The order that the route
// names are returned by Routes.Paths() determines the lookup order.
func NewHandler(c *Context, r Routes) *Handler {
	paths := make([]string, r.Len())
	i := 0
	for _, k := range r.Paths() {
		paths[i] = k
		i++
	}

	return &Handler{
		c:        c,
		resolver: httputil.NewResolver(paths),
		routes:   r,
	}
}

// ServeHTTP serves an HTTP request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf(helmhttp.LogAccess, r.Method, r.URL)
	route, err := h.resolver.Resolve(r)
	if err != nil {
		helmhttp.NotFound(w, r)
		return
	}

	fn, ok := h.routes.Get(route)
	if !ok {
		helmhttp.Fatal(w, r, "route %s missing", route)
	}

	if err := fn(w, r, h.c); err != nil {
		helmhttp.Fatal(w, r, err.Error())
	}
}

// Routes defines a container for route-to-function mapping.
type Routes interface {
	Add(string, HandlerFunc)
	Get(string) (HandlerFunc, bool)
	Len() int
	Paths() []string
}

// NewRoutes creates a default implementation of a Routes.
//
// The ordering of routes is nonderministic.
func NewRoutes() Routes {
	return routeMap{}
}

type routeMap map[string]HandlerFunc

func (r routeMap) Add(name string, fn HandlerFunc) {
	r[name] = fn
}

func (r routeMap) Get(name string) (HandlerFunc, bool) {
	f, ok := r[name]
	return f, ok
}

func (r routeMap) Len() int {
	return len(r)
}

func (r routeMap) Paths() []string {
	b := make([]string, len(r))
	i := 0
	for k := range r {
		b[i] = k
		i++
	}
	return b
}
