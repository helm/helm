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

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/storage"
	"k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/version"
)

// Timestamper is a function that can provide a timestamp.
//
// If this is not set, the `time.Now()` function is used to generate
// timestamps. This may be overridden for testing.
type Timestamper func() time.Time

// Configuration injects the dependencies that all actions share.
type Configuration struct {
	//engine    Engine
	Discovery discovery.DiscoveryInterface

	// Releases stores records of releases.
	Releases *storage.Storage
	// KubeClient is a Kubernetes API client.
	KubeClient environment.KubeClient

	Log func(string, ...interface{})

	Timestamper Timestamper
}

// capabilities builds a Capabilities from discovery information.
func (c *Configuration) capabilities() (*chartutil.Capabilities, error) {
	sv, err := c.Discovery.ServerVersion()
	if err != nil {
		return nil, err
	}
	vs, err := GetVersionSet(c.Discovery)
	if err != nil {
		return nil, errors.Wrap(err, "could not get apiVersions from Kubernetes")
	}
	return &chartutil.Capabilities{
		APIVersions: vs,
		KubeVersion: sv,
		HelmVersion: version.GetBuildInfo(),
	}, nil
}

// Now generates a timestamp
//
// If the configuration has a Timestamper on it, that will be used.
// Otherwise, this will use time.Now().
func (c *Configuration) Now() time.Time {
	if c.Timestamper != nil {
		return c.Timestamper()
	}
	return time.Now()
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
