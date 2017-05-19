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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	goprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/tlsutil"
	"k8s.io/helm/pkg/version"
)

const (
	// tlsEnableEnvVar names the environment variable that enables TLS.
	tlsEnableEnvVar = "TILLER_TLS_ENABLE"
	// tlsVerifyEnvVar names the environment variable that enables
	// TLS, as well as certificate verification of the remote.
	tlsVerifyEnvVar = "TILLER_TLS_VERIFY"
	// tlsCertsEnvVar names the environment variable that points to
	// the directory where Tiller's TLS certificates are located.
	tlsCertsEnvVar = "TILLER_TLS_CERTS"
)

const (
	storageMemory    = "memory"
	storageConfigMap = "configmap"
)

// rootServer is the root gRPC server.
//
// Each gRPC service registers itself to this server during init().
var rootServer *grpc.Server

// env is the default environment.
//
// Any changes to env should be done before rootServer.Serve() is called.
var env = environment.New()

var logger *log.Logger

var (
	grpcAddr             = ":44134"
	probeAddr            = ":44135"
	traceAddr            = ":44136"
	enableTracing        = false
	store                = storageConfigMap
	remoteReleaseModules = false
)

var (
	tlsEnable  bool
	tlsVerify  bool
	keyFile    string
	certFile   string
	caCertFile string
)

const globalUsage = `The Kubernetes Helm server.

Tiller is the server for Helm. It provides in-cluster resource management.

By default, Tiller listens for gRPC connections on port 44134.
`

func addFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&grpcAddr, "listen", "l", ":44134", "address:port to listen on")
	flags.StringVar(&store, "storage", storageConfigMap, "storage driver to use. One of 'configmap' or 'memory'")
	flags.BoolVar(&enableTracing, "trace", false, "enable rpc tracing")
	flags.BoolVar(&remoteReleaseModules, "experimental-release", false, "enable experimental release modules")

	flags.BoolVar(&tlsEnable, "tls", tlsEnableEnvVarDefault(), "enable TLS")
	flags.BoolVar(&tlsVerify, "tls-verify", tlsVerifyEnvVarDefault(), "enable TLS and verify remote certificate")
	flags.StringVar(&keyFile, "tls-key", tlsDefaultsFromEnv("tls-key"), "path to TLS private key file")
	flags.StringVar(&certFile, "tls-cert", tlsDefaultsFromEnv("tls-cert"), "path to TLS certificate file")
	flags.StringVar(&caCertFile, "tls-ca-cert", tlsDefaultsFromEnv("tls-ca-cert"), "trust certificates signed by this CA")
}

func initLog() {
	if enableTracing {
		log.SetFlags(log.Lshortfile)
	}
	logger = newLogger("main")
}

func main() {
	root := &cobra.Command{
		Use:   "tiller",
		Short: "The Kubernetes Helm server.",
		Long:  globalUsage,
		Run:   start,
		PreRun: func(_ *cobra.Command, _ []string) {
			initLog()
		},
	}
	addFlags(root.Flags())

	if err := root.Execute(); err != nil {
		logger.Fatal(err)
	}
}

func newLogger(prefix string) *log.Logger {
	if len(prefix) > 0 {
		prefix = fmt.Sprintf("[%s] ", prefix)
	}
	return log.New(os.Stderr, prefix, log.Flags())
}

func start(c *cobra.Command, args []string) {
	clientset, err := kube.New(nil).ClientSet()
	if err != nil {
		logger.Fatalf("Cannot initialize Kubernetes connection: %s", err)
	}

	switch store {
	case storageMemory:
		env.Releases = storage.Init(driver.NewMemory())
	case storageConfigMap:
		cfgmaps := driver.NewConfigMaps(clientset.Core().ConfigMaps(namespace()))
		cfgmaps.Log = newLogger("storage/driver").Printf

		env.Releases = storage.Init(cfgmaps)
		env.Releases.Log = newLogger("storage").Printf
	}

	kubeClient := kube.New(nil)
	kubeClient.Log = newLogger("kube").Printf
	env.KubeClient = kubeClient

	if tlsEnable || tlsVerify {
		opts := tlsutil.Options{CertFile: certFile, KeyFile: keyFile}
		if tlsVerify {
			opts.CaCertFile = caCertFile
		}
	}

	var opts []grpc.ServerOption
	if tlsEnable || tlsVerify {
		cfg, err := tlsutil.ServerConfig(tlsOptions())
		if err != nil {
			logger.Fatalf("Could not create server TLS configuration: %v", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(cfg)))
	}

	rootServer = tiller.NewServer(opts...)

	lstn, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Fatalf("Server died: %s", err)
	}

	logger.Printf("Starting Tiller %s (tls=%t)", version.GetVersion(), tlsEnable || tlsVerify)
	logger.Printf("GRPC listening on %s", grpcAddr)
	logger.Printf("Probes listening on %s", probeAddr)
	logger.Printf("Storage driver is %s", env.Releases.Name())

	if enableTracing {
		startTracing(traceAddr)
	}

	srvErrCh := make(chan error)
	probeErrCh := make(chan error)
	go func() {
		svc := tiller.NewReleaseServer(env, clientset, remoteReleaseModules)
		svc.Log = newLogger("tiller").Printf
		services.RegisterReleaseServiceServer(rootServer, svc)
		if err := rootServer.Serve(lstn); err != nil {
			srvErrCh <- err
		}
	}()

	go func() {
		mux := newProbesMux()

		// Register gRPC server to prometheus to initialized matrix
		goprom.Register(rootServer)
		addPrometheusHandler(mux)

		if err := http.ListenAndServe(probeAddr, mux); err != nil {
			probeErrCh <- err
		}
	}()

	select {
	case err := <-srvErrCh:
		logger.Fatalf("Server died: %s", err)
	case err := <-probeErrCh:
		logger.Printf("Probes server died: %s", err)
	}
}

// namespace returns the namespace of tiller
func namespace() string {
	if ns := os.Getenv("TILLER_NAMESPACE"); ns != "" {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return environment.DefaultTillerNamespace
}

func tlsOptions() tlsutil.Options {
	opts := tlsutil.Options{CertFile: certFile, KeyFile: keyFile}
	if tlsVerify {
		opts.CaCertFile = caCertFile
		opts.ClientAuth = tls.VerifyClientCertIfGiven
	}
	return opts
}

func tlsDefaultsFromEnv(name string) (value string) {
	switch certsDir := os.Getenv(tlsCertsEnvVar); name {
	case "tls-key":
		return filepath.Join(certsDir, "tls.key")
	case "tls-cert":
		return filepath.Join(certsDir, "tls.crt")
	case "tls-ca-cert":
		return filepath.Join(certsDir, "ca.crt")
	}
	return ""
}

func tlsEnableEnvVarDefault() bool { return os.Getenv(tlsEnableEnvVar) != "" }
func tlsVerifyEnvVarDefault() bool { return os.Getenv(tlsVerifyEnvVar) != "" }
