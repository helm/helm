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
	"github.com/kubernetes/helm/cmd/goexpander/expander"
	"github.com/kubernetes/helm/pkg/expansion"
	"github.com/kubernetes/helm/pkg/version"

	"flag"
	"log"
)

// interface that we are going to listen on
var address = flag.String("address", "", "Interface to listen on")

// port that we are going to listen on
var port = flag.Int("port", 8080, "Port to listen on")

func main() {
	flag.Parse()
	backend := expander.NewExpander()
	service := expansion.NewService(*address, *port, backend)
	log.Printf("Version: %s", version.Version)
	log.Printf("Listening on http://%s:%d/expand", *address, *port)
	log.Fatal(service.ListenAndServe())
}
