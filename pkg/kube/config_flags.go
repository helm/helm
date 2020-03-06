/*
Copyright The Helm Authors.

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

package kube

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"

	"helm.sh/helm/v3/internal/version"
)

var defaultCacheDir = filepath.Join(homedir.HomeDir(), ".kube", "http-cache")

// ConfigFlags composes the set of values necessary
// for obtaining a REST client config
type ConfigFlags struct {
	genericclioptions.ConfigFlags
}

// ToRESTConfig implements RESTClientGetter.
// Returns a REST client configuration based on a provided path
// to a .kubeconfig file, loading rules, and config flag overrides.
// Expects the AddFlags method to have been called.
func (f *ConfigFlags) ToRESTConfig() (*rest.Config, error) {
	config, err := f.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		return nil, err
	}
	config.UserAgent = version.GetUserAgent()
	return config, nil
}

// ToDiscoveryClient implements RESTClientGetter.
// Expects the AddFlags method to have been called.
// Returns a CachedDiscoveryInterface using a computed RESTConfig.
func (f *ConfigFlags) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	config.Burst = 100

	// retrieve a user-provided value for the "cache-dir"
	// defaulting to ~/.kube/http-cache if no user-value is given.
	httpCacheDir := defaultCacheDir
	if f.CacheDir != nil {
		httpCacheDir = *f.CacheDir
	}

	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery"), config.Host)
	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, time.Duration(10*time.Minute))
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive, so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/\.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}
