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

package release

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"helm.sh/helm/v4/pkg/release/common"
	rspb "helm.sh/helm/v4/pkg/release/v1"
)

func TestNewDefaultAccessor(t *testing.T) {
	// Testing the default implementation rather than NewAccessor which can be
	// overridden by developers.
	is := assert.New(t)

	// Create release
	info := &rspb.Info{Status: common.StatusDeployed, LastDeployed: time.Now().Add(1000)}
	labels := make(map[string]string)
	labels["foo"] = "bar"
	rel := &rspb.Release{
		Name:        "happy-cats",
		Version:     2,
		Info:        info,
		Labels:      labels,
		Namespace:   "default",
		ApplyMethod: "csa",
	}

	// newDefaultAccessor should not be called directly Instead, NewAccessor should be
	// called and it will call NewDefaultAccessor. NewAccessor can be changed to a
	// non-default accessor by a user so the test calls the default implementation.
	// The accessor provides a means to access data on resources that are different types
	// but have the same interface. Instead of properties, methods are used to access
	// information. Structs with properties are useful in Go when it comes to marshalling
	// and unmarshalling data (e.g. coming and going from JSON or YAML). But, structs
	// can't be used with interfaces. The accessors enable access to the underlying data
	// in a manner that works with Go interfaces.
	accessor, err := newDefaultAccessor(rel)
	is.NoError(err)

	// Verify information
	is.Equal(rel.Name, accessor.Name())
	is.Equal(rel.Namespace, accessor.Namespace())
	is.Equal(rel.Version, accessor.Version())
	is.Equal(rel.ApplyMethod, accessor.ApplyMethod())
	is.Equal(rel.Labels, accessor.Labels())
}
