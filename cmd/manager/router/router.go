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
	helmhttp "github.com/kubernetes/deployment-manager/pkg/httputil"
)

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
	routes   map[string]HandlerFunc
	paths    []string
}

// NewHandler creates a new Handler.
//
// Routes cannot be modified after construction. The order that the route
// names are returned by Routes.Paths() determines the lookup order.
func NewHandler(c *Context) *Handler {
	return &Handler{
		c:        c,
		resolver: httputil.NewResolver([]string{}),
		routes:   map[string]HandlerFunc{},
		paths:    []string{},
	}
}

// Add a route to a handler.
//
// The route name is "VERB /ENPOINT/PATH", e.g. "GET /foo".
func (h *Handler) Add(route string, fn HandlerFunc) {
	h.routes[route] = fn
	h.paths = append(h.paths, route)
	h.resolver = httputil.NewResolver(h.paths)
}

// ServeHTTP serves an HTTP request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf(helmhttp.LogAccess, r.Method, r.URL)
	route, err := h.resolver.Resolve(r)
	if err != nil {
		helmhttp.NotFound(w, r)
		return
	}

	fn, ok := h.routes[route]
	if !ok {
		helmhttp.Fatal(w, r, "route %s missing", route)
	}

	if err := fn(w, r, h.c); err != nil {
		helmhttp.Fatal(w, r, err.Error())
	}
}
