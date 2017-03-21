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
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/graymeta/stow"
	"github.com/graymeta/stow/azure"
	gcs "github.com/graymeta/stow/google"
	"github.com/graymeta/stow/s3"
	"github.com/graymeta/stow/swift"
	"github.com/spf13/cobra"
	kberrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"

	rapi "k8s.io/helm/api"
	rcs "k8s.io/helm/client/clientset"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/storage/driver"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/version"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

const (
	storageMemory         = "memory"
	storageConfigMap      = "configmap"
	storageInlineTPR      = "inline-tpr"
	storageObjectStoreTPR = "object-store-tpr"
)

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

	storageProvider          string
	s3ConfigAccessKeyID      string
	s3ConfigEndpoint         string
	s3ConfigRegion           string
	s3ConfigSecretKey        string
	gcsConfigJSONKeyPath     string
	gcsConfigProjectId       string
	gcsConfigScopes          string
	azureConfigAccount       string
	azureConfigKey           string
	swiftConfigKey           string
	swiftConfigTenantAuthURL string
	swiftConfigTenantName    string
	swiftConfigUsername      string

	container     string
	storagePrefix string
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

	p.StringVar(&storageProvider, "provider", "", "Cloud storage provider")

	p.StringVar(&s3ConfigAccessKeyID, s3.Kind+"."+s3.ConfigAccessKeyID, "", "S3 config access key id")
	p.StringVar(&s3ConfigEndpoint, s3.Kind+"."+s3.ConfigEndpoint, "", "S3 config endpoint")
	p.StringVar(&s3ConfigRegion, s3.Kind+"."+s3.ConfigRegion, "", "S3 config region")
	p.StringVar(&s3ConfigSecretKey, s3.Kind+"."+s3.ConfigSecretKey, "", "S3 config secret key")

	p.StringVar(&gcsConfigJSONKeyPath, gcs.Kind+".json_key_path", "", "GCS config json key path")
	p.StringVar(&gcsConfigProjectId, gcs.Kind+"."+gcs.ConfigProjectId, "", "GCS config project id")
	p.StringVar(&gcsConfigScopes, gcs.Kind+"."+gcs.ConfigScopes, "", "GCS config scopes")

	p.StringVar(&azureConfigAccount, azure.Kind+"."+azure.ConfigAccount, "", "Azure config account")
	p.StringVar(&azureConfigKey, azure.Kind+"."+azure.ConfigKey, "", "Azure config key")

	p.StringVar(&swiftConfigKey, swift.Kind+"."+swift.ConfigKey, "", "Swift config key")
	p.StringVar(&swiftConfigTenantAuthURL, swift.Kind+"."+swift.ConfigTenantAuthURL, "", "Swift teanant auth url")
	p.StringVar(&swiftConfigTenantName, swift.Kind+"."+swift.ConfigTenantName, "", "Swift tenant name")
	p.StringVar(&swiftConfigUsername, swift.Kind+"."+swift.ConfigUsername, "", "Swift username")

	p.StringVar(&container, "container", "", "Name of container")
	p.StringVar(&storagePrefix, "storage-prefix", "tiller", "Prefix to container key where release data is stored")

	if err := rootCommand.Execute(); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func start(c *cobra.Command, args []string) {
	kc := kube.New(nil)
	clientcfg, err := kc.ClientConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot initialize Kubernetes connection: %s\n", err)
		os.Exit(1)
	}
	clientset, err := kc.ClientSet()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot initialize Kubernetes connection: %s\n", err)
		os.Exit(1)
	}

	switch store {
	case storageMemory:
		env.Releases = storage.Init(driver.NewMemory())
	case storageConfigMap:
		env.Releases = storage.Init(driver.NewConfigMaps(clientset.Core().ConfigMaps(namespace())))
	case storageInlineTPR:
		ensureResource(clientset)
		cs := rcs.NewExtensionsForConfigOrDie(clientcfg)
		env.Releases = storage.Init(driver.NewReleases(cs.Release(namespace())))
	case storageObjectStoreTPR:
		ensureResource(clientset)
		stowCfg := stow.ConfigMap{}
		switch storageProvider {
		case s3.Kind:
			if s3ConfigAccessKeyID != "" {
				stowCfg[s3.ConfigAccessKeyID] = s3ConfigAccessKeyID
			}
			if s3ConfigEndpoint != "" {
				stowCfg[s3.ConfigEndpoint] = s3ConfigEndpoint
			}
			if s3ConfigRegion != "" {
				stowCfg[s3.ConfigRegion] = s3ConfigRegion
			}
			if s3ConfigSecretKey != "" {
				stowCfg[s3.ConfigSecretKey] = s3ConfigSecretKey
			}
		case gcs.Kind:
			if gcsConfigJSONKeyPath != "" {
				jsonKey, err := ioutil.ReadFile(gcsConfigJSONKeyPath)
				fmt.Fprintf(os.Stderr, "Cannot read json key file: %s\n", err)
				os.Exit(1)
				stowCfg[gcs.ConfigJSON] = string(jsonKey)
			}
			if gcsConfigProjectId != "" {
				stowCfg[gcs.ConfigProjectId] = gcsConfigProjectId
			}
			if gcsConfigScopes != "" {
				stowCfg[gcs.ConfigScopes] = gcsConfigScopes
			}
		case azure.Kind:
			if azureConfigAccount != "" {
				stowCfg[azure.ConfigAccount] = azureConfigAccount
			}
			if azureConfigKey != "" {
				stowCfg[azure.ConfigKey] = azureConfigKey
			}
		case swift.Kind:
			if swiftConfigKey != "" {
				stowCfg[swift.ConfigKey] = swiftConfigKey
			}
			if swiftConfigTenantAuthURL != "" {
				stowCfg[swift.ConfigTenantAuthURL] = swiftConfigTenantAuthURL
			}
			if swiftConfigTenantName != "" {
				stowCfg[swift.ConfigTenantName] = swiftConfigTenantName
			}
			if swiftConfigUsername != "" {
				stowCfg[swift.ConfigUsername] = swiftConfigUsername
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown provider: %s\n", storageProvider)
			os.Exit(1)
		}
		loc, err := stow.Dial(storageProvider, stowCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot connect to object store: %s\n", err)
			os.Exit(1)
		}
		c, err := loc.Container(container)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot find container: %s\n", err)
			os.Exit(1)
		}
		cs := rcs.NewExtensionsForConfigOrDie(clientcfg)
		env.Releases = storage.Init(driver.NewObjectStoreReleases(cs.Release(namespace()), c, storagePrefix))
	}

	lstn, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server died: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting Tiller %s\n", version.GetVersion())
	fmt.Printf("GRPC listening on %s\n", grpcAddr)
	fmt.Printf("Probes listening on %s\n", probeAddr)
	fmt.Printf("Storage driver is %s\n", env.Releases.Name())

	if enableTracing {
		startTracing(traceAddr)
	}

	srvErrCh := make(chan error)
	probeErrCh := make(chan error)
	go func() {
		svc := tiller.NewReleaseServer(env, clientset)
		services.RegisterReleaseServiceServer(rootServer, svc)
		if err := rootServer.Serve(lstn); err != nil {
			srvErrCh <- err
		}
	}()

	go func() {
		mux := newProbesMux()
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

func ensureResource(clientset *internalclientset.Clientset) {
	_, err := clientset.Extensions().ThirdPartyResources().Get("release." + rapi.V1alpha1SchemeGroupVersion.Group)
	if kberrs.IsNotFound(err) {
		tpr := &extensions.ThirdPartyResource{
			TypeMeta: unversioned.TypeMeta{
				APIVersion: "extensions/v1alpha1",
				Kind:       "ThirdPartyResource",
			},
			ObjectMeta: api.ObjectMeta{
				Name: "release." + rapi.V1alpha1SchemeGroupVersion.Group,
			},
			Versions: []extensions.APIVersion{
				{
					Name: rapi.V1alpha1SchemeGroupVersion.Version,
				},
			},
		}
		_, err := clientset.Extensions().ThirdPartyResources().Create(tpr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create third party resource: %s\n", err)
			os.Exit(1)
		}
	}
}
