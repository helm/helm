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

package main // import "k8s.io/helm/cmd/tiller"

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"k8s.io/helm/cmd/tiller/environment"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
)

const (
	storageMemory    = "memory"
	storageConfigMap = "configmap"
)

// rootServer is the root gRPC server.
//
// Each gRPC service registers itself to this server during init().
var rootServer = grpc.NewServer()

// env is the default environment.
//
// Any changes to env should be done before rootServer.Serve() is called.
var env = environment.New()

var (
	addr  = ":44134"
	probe = ":44135"
	store = storageConfigMap
)

const globalUsage = `The Kubernetes Helm server.

Tiller is the server for Helm. It provides in-cluster resource management.

By default, Tiller listens for gRPC connections on port 44134.
`

var rootCommand = &cobra.Command{
	Use:   "tiller",
	Short: "The Kubernetes Helm server.",
	Long:  globalUsage,
	Run:   start,
}

func main() {
	pf := rootCommand.PersistentFlags()
	pf.StringVarP(&addr, "listen", "l", ":44134", "The address:port to listen on")
	pf.StringVar(&store, "storage", storageConfigMap, "The storage driver to use. One of 'configmap' or 'memory'")
	rootCommand.Execute()
}

func start(c *cobra.Command, args []string) {
	switch store {
	case storageMemory:
		env.Releases = storage.Init(driver.NewMemory())
	case storageConfigMap:
		c, err := env.KubeClient.APIClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot initialize Kubernetes connection: %s", err)
		}
		env.Releases = storage.Init(driver.NewConfigMaps(c.ConfigMaps(environment.TillerNamespace)))
	}

	lstn, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Tiller is running on %s\n", addr)
	fmt.Printf("Tiller probes server is running on %s\n", probe)
	fmt.Printf("Storage driver is %s\n", env.Releases.Name())

	srvErrCh := make(chan error)
	probeErrCh := make(chan error)
	go func() {
		if err := rootServer.Serve(lstn); err != nil {
			srvErrCh <- err
		}
	}()

	go func() {
		mux := newProbesMux()
		if err := http.ListenAndServe(probe, mux); err != nil {
			probeErrCh <- err
		}
	}()

	select {
	case err := <-srvErrCh:
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	case err := <-probeErrCh:
		fmt.Fprintf(os.Stderr, "Probes server died: %s\n", err)
	}
}
