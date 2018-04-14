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

	"k8s.io/helm/pkg/proto/hapi/services"
)

func TestGetReleaseContent(t *testing.T) {
	rs := rsFixture()
	rel := releaseStub()
	if err := rs.env.Releases.Create(rel); err != nil {
		t.Fatalf("Could not store mock release: %s", err)
	}

	res, err := rs.GetReleaseContent(&services.GetReleaseContentRequest{Name: rel.Name, Version: 1})
	if err != nil {
		t.Errorf("Error getting release content: %s", err)
	}

	if res.Chart.Metadata.Name != rel.Chart.Metadata.Name {
		t.Errorf("Expected %q, got %q", rel.Chart.Metadata.Name, res.Chart.Metadata.Name)
	}
}
