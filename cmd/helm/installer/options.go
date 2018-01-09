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

	"k8s.io/api/core/v1"
	"k8s.io/helm/pkg/strvals"
	"k8s.io/helm/pkg/version"
)

const defaultImage = "gcr.io/kubernetes-helm/tiller"

// Options control how to install Tiller into a cluster, upgrade, and uninstall Tiller from a cluster.
type Options struct {
	// EnableTLS instructs Tiller to serve with TLS enabled.
	//
	// Implied by VerifyTLS. If set the TLSKey and TLSCert are required.
	EnableTLS bool

	// VerifyTLS instructs Tiller to serve with TLS enabled verify remote certificates.
	//
	// If set TLSKey, TLSCert, TLSCaCert are required.
	VerifyTLS bool

	// UseCanary indicates that Tiller should deploy using the latest Tiller image.
	UseCanary bool

	// Namespace is the Kubernetes namespace to use to deploy Tiller.
	Namespace string

	// ServiceAccount is the Kubernetes service account to add to Tiller.
	ServiceAccount string

	// Force allows to force upgrading tiller if deployed version is greater than current version
	ForceUpgrade bool

	// ImageSpec indentifies the image Tiller will use when deployed.
	//
	// Valid if and only if UseCanary is false.
	ImageSpec string

	// TLSKeyFile identifies the file containing the pem encoded TLS private
	// key Tiller should use.
	//
	// Required and valid if and only if EnableTLS or VerifyTLS is set.
	TLSKeyFile string

	// TLSCertFile identifies the file containing the pem encoded TLS
	// certificate Tiller should use.
	//
	// Required and valid if and only if EnableTLS or VerifyTLS is set.
	TLSCertFile string

	// TLSCaCertFile identifies the file containing the pem encoded TLS CA
	// certificate Tiller should use to verify remotes certificates.
	//
	// Required and valid if and only if VerifyTLS is set.
	TLSCaCertFile string

	// EnableHostNetwork installs Tiller with net=host.
	EnableHostNetwork bool

	// MaxHistory sets the maximum number of release versions stored per release.
	//
	// Less than or equal to zero means no limit.
	MaxHistory int

	// NodeSelectors determine which nodes Tiller can land on.
	NodeSelectors string

	// Output dumps the Tiller manifest in the specified format (e.g. JSON) but skips Helm/Tiller installation.
	Output OutputFormat

	// Set merges additional values into the Tiller Deployment manifest.
	Values []string
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

// valuesMap returns user set values in map format
func (opts *Options) valuesMap(m map[string]interface{}) (map[string]interface{}, error) {
	for _, skv := range opts.Values {
		if err := strvals.ParseInto(skv, m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// OutputFormat defines valid values for init output (json, yaml)
type OutputFormat string

// String returns the string value of the OutputFormat
func (f *OutputFormat) String() string {
	return string(*f)
}

// Type returns the string value of the OutputFormat
func (f *OutputFormat) Type() string {
	return "OutputFormat"
}

const (
	fmtJSON OutputFormat = "json"
	fmtYAML OutputFormat = "yaml"
)

// Set validates and sets the value of the OutputFormat
func (f *OutputFormat) Set(s string) error {
	for _, of := range []OutputFormat{fmtJSON, fmtYAML} {
		if s == string(of) {
			*f = of
			return nil
		}
	}
	return fmt.Errorf("unknown output format %q", s)
}
