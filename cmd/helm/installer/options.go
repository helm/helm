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

package installer // import "k8s.io/helm/cmd/helm/installer"

import (
	"fmt"

	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/helm/pkg/version"
)

const defaultImage = "gcr.io/kubernetes-helm/tiller"

// Options control how to install tiller into a cluster, upgrade, and uninstall tiller from a cluster.
type Options struct {
	// EnableTLS instructs tiller to serve with TLS enabled.
	//
	// Implied by VerifyTLS. If set the TLSKey and TLSCert are required.
	EnableTLS bool

	// VerifyTLS instructs tiller to serve with TLS enabled verify remote certificates.
	//
	// If set TLSKey, TLSCert, TLSCaCert are required.
	VerifyTLS bool

	// UseCanary indicates that tiller should deploy using the latest tiller image.
	UseCanary bool

	// Namespace is the kubernetes namespace to use to deploy tiller.
	Namespace string

	// ServiceAccount is the Kubernetes service account to add to tiller
	ServiceAccount string

	// ImageSpec indentifies the image tiller will use when deployed.
	//
	// Valid if and only if UseCanary is false.
	ImageSpec string

	// TLSKeyFile identifies the file containing the pem encoded TLS private
	// key tiller should use.
	//
	// Required and valid if and only if EnableTLS or VerifyTLS is set.
	TLSKeyFile string

	// TLSCertFile identifies the file containing the pem encoded TLS
	// certificate tiller should use.
	//
	// Required and valid if and only if EnableTLS or VerifyTLS is set.
	TLSCertFile string

	// TLSCaCertFile identifies the file containing the pem encoded TLS CA
	// certificate tiller should use to verify remotes certificates.
	//
	// Required and valid if and only if VerifyTLS is set.
	TLSCaCertFile string

	// EnableHostNetwork installs Tiller with net=host
	EnableHostNetwork bool
}

func (opts *Options) selectImage() string {
	switch {
	case opts.UseCanary:
		return defaultImage + ":canary"
	case opts.ImageSpec == "":
		return fmt.Sprintf("%s:%s", defaultImage, version.Version)
	default:
		return opts.ImageSpec
	}
}

func (opts *Options) pullPolicy() v1.PullPolicy {
	if opts.UseCanary {
		return v1.PullAlways
	}
	return v1.PullIfNotPresent
}

func (opts *Options) tls() bool { return opts.EnableTLS || opts.VerifyTLS }
