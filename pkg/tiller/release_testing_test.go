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

package tiller

import (
	"testing"

	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/proto/hapi/services"
)

func TestRunReleaseTest(t *testing.T) {
	rs := rsFixture()
	rel := namedReleaseStub("nemo", release.Status_DEPLOYED)
	rs.env.Releases.Create(rel)

	req := &services.TestReleaseRequest{Name: "nemo", Timeout: 2}
	err := rs.RunReleaseTest(req, mockRunReleaseTestServer{})
	if err != nil {
		t.Fatalf("failed to run release tests on %s: %s", rel.Name, err)
	}
}
