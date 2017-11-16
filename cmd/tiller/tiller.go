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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	goprom "github.com/grpc-ecosystem/go-grpc-prometheus"
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
	// historyMaxEnvVar is the name of the env var for setting max history.
	historyMaxEnvVar = "TILLER_HISTORY_MAX"

	storageMemory    = "memory"
	storageConfigMap = "configmap"
	storageSecret    = "secret"

	probeAddr = ":44135"
	traceAddr = ":44136"

	// defaultMaxHistory sets the maximum number of releases to 0: unlimited
	defaultMaxHistory = 0
)

var (
	grpcAddr             = flag.String("listen", ":44134", "address:port to listen on")
	enableTracing        = flag.Bool("trace", false, "enable rpc tracing")
	store                = flag.String("storage", storageConfigMap, "storage driver to use. One of 'configmap', 'memory', or 'secret'")
	remoteReleaseModules = flag.Bool("experimental-release", false, "enable experimental release modules")
	tlsEnable            = flag.Bool("tls", tlsEnableEnvVarDefault(), "enable TLS")
	tlsVerify            = flag.Bool("tls-verify", tlsVerifyEnvVarDefault(), "enable TLS and verify remote certificate")
	keyFile              = flag.String("tls-key", tlsDefaultsFromEnv("tls-key"), "path to TLS private key file")
	certFile             = flag.String("tls-cert", tlsDefaultsFromEnv("tls-cert"), "path to TLS certificate file")
	caCertFile           = flag.String("tls-ca-cert", tlsDefaultsFromEnv("tls-ca-cert"), "trust certificates signed by this CA")
	maxHistory           = flag.Int("history-max", historyMaxFromEnv(), "maximum number of releases kept in release history, with 0 meaning no limit")

	// rootServer is the root gRPC server.
	//
	// Each gRPC service registers itself to this server during init().
	rootServer *grpc.Server

	// env is the default environment.
	//
	// Any changes to env should be done before rootServer.Serve() is called.
	env = environment.New()

	logger *log.Logger
)

func main() {
	flag.Parse()

	if *enableTracing {
		log.SetFlags(log.Lshortfile)
	}
	logger = newLogger("main")

	start()
}

func start() {

	clientset, err := kube.New(nil).ClientSet()
	if err != nil {
		logger.Fatalf("Cannot initialize Kubernetes connection: %s", err)
	}

	switch *store {
	case storageMemory:
		env.Releases = storage.Init(driver.NewMemory())
	case storageConfigMap:
		cfgmaps := driver.NewConfigMaps(clientset.Core().ConfigMaps(namespace()))
		cfgmaps.Log = newLogger("storage/driver").Printf

		env.Releases = storage.Init(cfgmaps)
		env.Releases.Log = newLogger("storage").Printf
	case storageSecret:
		secrets := driver.NewSecrets(clientset.Core().Secrets(namespace()))
		secrets.Log = newLogger("storage/driver").Printf

		env.Releases = storage.Init(secrets)
		env.Releases.Log = newLogger("storage").Printf
	}

	if *maxHistory > 0 {
		env.Releases.MaxHistory = *maxHistory
	}

	kubeClient := kube.New(nil)
	kubeClient.Log = newLogger("kube").Printf
	env.KubeClient = kubeClient

	if *tlsEnable || *tlsVerify {
		opts := tlsutil.Options{CertFile: *certFile, KeyFile: *keyFile}
		if *tlsVerify {
			opts.CaCertFile = *caCertFile
		}
	}

	var opts []grpc.ServerOption
	if *tlsEnable || *tlsVerify {
		cfg, err := tlsutil.ServerConfig(tlsOptions())
		if err != nil {
			logger.Fatalf("Could not create server TLS configuration: %v", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(cfg)))
	}

	rootServer = tiller.NewServer(opts...)

	lstn, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		logger.Fatalf("Server died: %s", err)
	}

	logger.Printf("Starting Tiller %s (tls=%t)", version.GetVersion(), *tlsEnable || *tlsVerify)
	logger.Printf("GRPC listening on %s", *grpcAddr)
	logger.Printf("Probes listening on %s", probeAddr)
	logger.Printf("Storage driver is %s", env.Releases.Name())
	logger.Printf("Max history per release is %d", *maxHistory)

	if *enableTracing {
		startTracing(traceAddr)
	}

	srvErrCh := make(chan error)
	probeErrCh := make(chan error)
	go func() {
		svc := tiller.NewReleaseServer(env, clientset, *remoteReleaseModules)
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

func newLogger(prefix string) *log.Logger {
	if len(prefix) > 0 {
		prefix = fmt.Sprintf("[%s] ", prefix)
	}
	return log.New(os.Stderr, prefix, log.Flags())
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
	opts := tlsutil.Options{CertFile: *certFile, KeyFile: *keyFile}
	if *tlsVerify {
		opts.CaCertFile = *caCertFile

		// We want to force the client to not only provide a cert, but to
		// provide a cert that we can validate.
		// http://www.bite-code.com/2015/06/25/tls-mutual-auth-in-golang/
		opts.ClientAuth = tls.RequireAndVerifyClientCert
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

func historyMaxFromEnv() int {
	val := os.Getenv(historyMaxEnvVar)
	if val == "" {
		return defaultMaxHistory
	}
	ret, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("Invalid max history %q. Defaulting to 0.", val)
		return defaultMaxHistory
	}
	return ret
}

func tlsEnableEnvVarDefault() bool { return os.Getenv(tlsEnableEnvVar) != "" }
func tlsVerifyEnvVarDefault() bool { return os.Getenv(tlsVerifyEnvVar) != "" }
