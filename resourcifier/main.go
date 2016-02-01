/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package main

import (
	"github.com/kubernetes/deployment-manager/util"
	"github.com/kubernetes/deployment-manager/version"

	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Route defines a routing table entry to be registered with gorilla/mux.
type Route struct {
	Name        string
	Path        string
	Methods     string
	HandlerFunc http.HandlerFunc
	Type        string
}

var routes = []Route{
	{"HealthCheck", "/healthz", "GET", healthCheckHandlerFunc, ""},
}

// port to listen on
var port = flag.Int("port", 8080, "The port to listen on")

func main() {
	if !flag.Parsed() {
		flag.Parse()
	}

	router := mux.NewRouter()
	router.StrictSlash(true)
	for _, route := range routes {
		handler := http.Handler(http.HandlerFunc(route.HandlerFunc))
		switch route.Type {
		case "JSON":
			handler = handlers.ContentTypeHandler(handler, "application/json")
		case "":
			break
		default:
			log.Fatalf("invalid route type: %v", route)
		}

		r := router.NewRoute()
		r.Name(route.Name).
			Path(route.Path).
			Methods(route.Methods).
			Handler(handler)
	}

	address := fmt.Sprintf(":%d", *port)
	handler := handlers.CombinedLoggingHandler(os.Stderr, router)
	log.Printf("Version: %s", version.DeploymentManagerVersion)
	log.Printf("Listening on port %d...", *port)
	log.Fatal(http.ListenAndServe(address, handler))
}

func healthCheckHandlerFunc(w http.ResponseWriter, r *http.Request) {
	handler := "manager: get health"
	util.LogHandlerEntry(handler, r)
	util.LogHandlerExitWithText(handler, w, "OK", http.StatusOK)
}
