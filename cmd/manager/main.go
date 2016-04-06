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
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/kubernetes/helm/cmd/manager/router"
	"github.com/kubernetes/helm/pkg/httputil"
	"github.com/kubernetes/helm/pkg/version"
)

var (
	port              = flag.Int("port", 8080, "The port to listen on")
	maxLength         = flag.Int64("maxLength", 1024, "The maximum length (KB) of a template.")
	expanderPort      = flag.String("expanderPort", "8081", "The IP port of the default expander service.")
	expanderURL       = flag.String("expanderURL", "", "The URL for the default expander service.")
	deployerName      = flag.String("deployer", "resourcifier-service", "The DNS name of the deployer service.")
	deployerPort      = flag.String("deployerPort", "8082", "The IP port of the deployer service.")
	deployerURL       = flag.String("deployerURL", "", "The URL for the deployer service.")
	credentialFile    = flag.String("credentialFile", "", "Local file to use for credentials.")
	credentialSecrets = flag.Bool("credentialSecrets", true, "Use secrets for credentials.")
	mongoName         = flag.String("mongoName", "mongodb", "The DNS name of the mongodb service.")
	mongoPort         = flag.String("mongoPort", "27017", "The port of the mongodb service.")
	mongoAddress      = flag.String("mongoAddress", "mongodb:27017", "The address of the mongodb service.")
)

func main() {
	// Set up dependencies
	c := &router.Context{
		Config: parseFlags(),
	}

	if err := setupDependencies(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	httputil.DefaultEncoder.MaxReadLen = c.Config.MaxTemplateLength

	// Set up routes
	handler := router.NewHandler(c)
	registerDeploymentRoutes(c, handler)
	registerChartRepoRoutes(c, handler)

	// Now create a server.
	log.Printf("Starting Manager %s on %s", version.Version, c.Config.Address)
	if err := http.ListenAndServe(c.Config.Address, handler); err != nil {
		log.Printf("Server exited with error %s", err)
		os.Exit(1)
	}
}

func parseFlags() *router.Config {
	flag.Parse()
	return &router.Config{
		Address:           fmt.Sprintf(":%d", *port),
		MaxTemplateLength: *maxLength,
		ExpanderPort:      *expanderPort,
		ExpanderURL:       *expanderURL,
		DeployerName:      *deployerName,
		DeployerPort:      *deployerPort,
		DeployerURL:       *deployerURL,
		CredentialFile:    *credentialFile,
		CredentialSecrets: *credentialSecrets,
		MongoName:         *mongoName,
		MongoPort:         *mongoPort,
		MongoAddress:      *mongoAddress,
	}
}
