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
	"github.com/kubernetes/deployment-manager/cmd/expandybird/expander"
	"github.com/kubernetes/deployment-manager/cmd/expandybird/service"
	"github.com/kubernetes/deployment-manager/pkg/version"

	"flag"
	"fmt"
	"log"
	"net/http"

	restful "github.com/emicklei/go-restful"
)

// port that we are going to listen on
var port = flag.Int("port", 8080, "Port to listen on")

// path to expansion binary
var expansionBinary = flag.String("expansion_binary", "../../../expansion/expansion.py",
	"The path to the expansion binary that will be used to expand the template.")

func main() {
	flag.Parse()
	backend := expander.NewExpander(*expansionBinary)
	wrapper := service.NewService(service.NewExpansionHandler(backend))
	address := fmt.Sprintf(":%d", *port)
	container := restful.DefaultContainer
	server := &http.Server{
		Addr:    address,
		Handler: container,
	}

	wrapper.Register(container)
	log.Printf("Version: %s", version.DeploymentManagerVersion)
	log.Printf("Listening on %s...", address)
	log.Fatal(server.ListenAndServe())
}
