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

package action

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/tiller/environment"
)

// Timestamper is a function capable of producing a timestamp.Timestamper.
//
// By default, this is a time.Time function. This can be overridden for testing,
// though, so that timestamps are predictable.
var Timestamper = time.Now

// Configuration injects the dependencies that all actions share.
type Configuration struct {
	// Discovery contains a discovery client
	Discovery discovery.DiscoveryInterface

	// Releases stores records of releases.
	Releases *storage.Storage
	// KubeClient is a Kubernetes API client.
	KubeClient environment.KubeClient

	Capabilities *chartutil.Capabilities

	Log func(string, ...interface{})
}

// capabilities builds a Capabilities from discovery information.
func (c *Configuration) capabilities() *chartutil.Capabilities {
	if c.Capabilities == nil {
		return chartutil.DefaultCapabilities
	}
	return c.Capabilities
}

// Now generates a timestamp
//
// If the configuration has a Timestamper on it, that will be used.
// Otherwise, this will use time.Now().
func (c *Configuration) Now() time.Time {
	return Timestamper()
}

// GetVersionSet retrieves a set of available k8s API versions
func GetVersionSet(client discovery.ServerGroupsInterface) (chartutil.VersionSet, error) {
	groups, err := client.ServerGroups()
	if err != nil {
		return chartutil.DefaultVersionSet, err
	}

	// FIXME: The Kubernetes test fixture for cli appears to always return nil
	// for calls to Discovery().ServerGroups(). So in this case, we return
	// the default API list. This is also a safe value to return in any other
	// odd-ball case.
	if groups.Size() == 0 {
		return chartutil.DefaultVersionSet, nil
	}

	versions := metav1.ExtractGroupVersions(groups)
	return chartutil.NewVersionSet(versions...), nil
}
