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

var rootCommand = &cobra.Command{
	Use:   "tiller",
	Short: "The Kubernetes Helm server.",
	Long:  globalUsage,
	Run:   start,
}

func init() {
	log.SetFlags(log.Flags() | log.Lshortfile)
}

func main() {
	p := rootCommand.PersistentFlags()
	p.StringVarP(&grpcAddr, "listen", "l", ":44134", "address:port to listen on")
	p.StringVar(&store, "storage", storageConfigMap, "storage driver to use. One of 'configmap' or 'memory'")
	p.BoolVar(&enableTracing, "trace", false, "enable rpc tracing")
	p.BoolVar(&remoteReleaseModules, "experimental-release", false, "enable experimental release modules")

	p.BoolVar(&tlsEnable, "tls", tlsEnableEnvVarDefault(), "enable TLS")
	p.BoolVar(&tlsVerify, "tls-verify", tlsVerifyEnvVarDefault(), "enable TLS and verify remote certificate")
	p.StringVar(&keyFile, "tls-key", tlsDefaultsFromEnv("tls-key"), "path to TLS private key file")
	p.StringVar(&certFile, "tls-cert", tlsDefaultsFromEnv("tls-cert"), "path to TLS certificate file")
	p.StringVar(&caCertFile, "tls-ca-cert", tlsDefaultsFromEnv("tls-ca-cert"), "trust certificates signed by this CA")

	if err := rootCommand.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func start(c *cobra.Command, args []string) {
	clientset, err := kube.New(nil).ClientSet()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot initialize Kubernetes connection: %s\n", err)
		os.Exit(1)
	}

	switch store {
	case storageMemory:
		env.Releases = storage.Init(driver.NewMemory())
	case storageConfigMap:
		env.Releases = storage.Init(driver.NewConfigMaps(clientset.Core().ConfigMaps(namespace())))
	}

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
			fmt.Fprintf(os.Stderr, "Could not create server TLS configuration: %v\n", err)
			os.Exit(1)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(cfg)))
	}

	rootServer = tiller.NewServer(opts...)

	lstn, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting Tiller %s (tls=%t)\n", version.GetVersion(), tlsEnable || tlsVerify)
	fmt.Printf("GRPC listening on %s\n", grpcAddr)
	fmt.Printf("Probes listening on %s\n", probeAddr)
	fmt.Printf("Storage driver is %s\n", env.Releases.Name())

	if enableTracing {
		startTracing(traceAddr)
	}

	srvErrCh := make(chan error)
	probeErrCh := make(chan error)
	go func() {
		svc := tiller.NewReleaseServer(env, clientset, remoteReleaseModules)
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
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	case err := <-probeErrCh:
		fmt.Fprintf(os.Stderr, "Probes server died: %s\n", err)
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
