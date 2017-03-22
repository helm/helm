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
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	log "github.com/Sirupsen/logrus"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/logutil"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/version"
)

const (
	storageMemory    = "memory"
	storageConfigMap = "configmap"
)

// Variable for the string log level
var level string

// rootServer is the root gRPC server.
//
// Each gRPC service registers itself to this server during init().
var rootServer = tiller.NewServer()

// env is the default environment.
//
// Any changes to env should be done before rootServer.Serve() is called.
var env = environment.New()

var (
	grpcAddr      = ":44134"
	probeAddr     = ":44135"
	traceAddr     = ":44136"
	enableTracing = false
	store         = storageConfigMap
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

// logger with base fields for this package
var logger *log.Entry

func init() {
	logger = log.WithFields(log.Fields{
		"_package": "main",
	})
	// Makes the logger use a full timestamp
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
}

func main() {
	p := rootCommand.PersistentFlags()
	p.StringVarP(&grpcAddr, "listen", "l", ":44134", "address:port to listen on")
	p.StringVar(&store, "storage", storageConfigMap, "storage driver to use. One of 'configmap' or 'memory'")
	p.BoolVar(&enableTracing, "trace", false, "enable rpc tracing")
	p.StringVar(&level, "log-level", "INFO", "level to log at. One of 'ERROR', 'INFO', 'DEBUG'")

	if err := rootCommand.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func start(c *cobra.Command, args []string) {
	// Set the log level first
	logLevel, err := logutil.GetLevel(level)
	if err != nil {
		logger.WithFields(log.Fields{
			"_module":  "tiller",
			"_context": "start",
			"error":    err,
		}).Error("Unable to get log level")
		os.Exit(1)
	}
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
		"level":    level,
	}).Debug("Setting log level")
	log.SetLevel(logLevel)
	clientset, err := kube.New(nil).ClientSet()
	if err != nil {
		logger.WithFields(log.Fields{
			"_module":  "tiller",
			"_context": "start",
			"error":    err,
		}).Error("Cannot initialize Kubernetes connection")
		os.Exit(1)
	}
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
		"driver":   store,
	}).Debug("Setting storage driver")
	switch store {
	case storageMemory:
		env.Releases = storage.Init(driver.NewMemory())
	case storageConfigMap:
		env.Releases = storage.Init(driver.NewConfigMaps(clientset.Core().ConfigMaps(namespace())))
	}

	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
		"address":  grpcAddr,
	}).Debug("Starting GRPC server")
	lstn, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.WithFields(log.Fields{
			"_module":  "tiller",
			"_context": "start",
			"error":    err,
		}).Error("Server died")
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	}

	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
	}).Infof("Tiller %s running", version.GetVersion())
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
	}).Infof("GRPC listening on %s", grpcAddr)
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
	}).Infof("Probes listening on %s", probeAddr)
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
	}).Infof("Storage driver is %s", env.Releases.Name())

	if enableTracing {
		logger.WithFields(log.Fields{
			"_module":  "tiller",
			"_context": "start",
			"address":  grpcAddr,
		}).Debug("Starting GRPC trace")
		startTracing(traceAddr)
	}

	srvErrCh := make(chan error)
	probeErrCh := make(chan error)
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
	}).Debug("Starting release server")
	go func() {
		svc := tiller.NewReleaseServer(env, clientset)
		services.RegisterReleaseServiceServer(rootServer, svc)
		if err := rootServer.Serve(lstn); err != nil {
			srvErrCh <- err
		}
	}()
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "start",
	}).Debug("Starting probes server")
	go func() {
		mux := newProbesMux()
		if err := http.ListenAndServe(probeAddr, mux); err != nil {
			probeErrCh <- err
		}
	}()

	select {
	case err := <-srvErrCh:
		logger.WithFields(log.Fields{
			"_module":  "tiller",
			"_context": "start",
			"error":    err,
		}).Error("Release server died")
		os.Exit(1)
	case err := <-probeErrCh:
		logger.WithFields(log.Fields{
			"_module":  "tiller",
			"_context": "start",
			"error":    err,
		}).Error("Probes server died")
	}
}

// namespace returns the namespace of tiller
func namespace() string {
	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "namespace",
	}).Debug("Getting tiller namespace")
	if ns := os.Getenv("TILLER_NAMESPACE"); ns != "" {
		logger.WithFields(log.Fields{
			"_module":   "tiller",
			"_context":  "namespace",
			"namespace": ns,
		}).Debug("Found TILLER_NAMESPACE environment variable")
		return ns
	}

	logger.WithFields(log.Fields{
		"_module":  "tiller",
		"_context": "namespace",
	}).Debug("No namespace variable set. Attempting to get namespace from service account token")
	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			logger.WithFields(log.Fields{
				"_module":   "tiller",
				"_context":  "namespace",
				"namespace": ns,
			}).Debug("Found service account token namespace")
			return ns
		}
	}

	logger.WithFields(log.Fields{
		"_module":   "tiller",
		"_context":  "namespace",
		"namespace": environment.DefaultTillerNamespace,
	}).Debug("No namespaces found. Returning default namespace")

	return environment.DefaultTillerNamespace
}
