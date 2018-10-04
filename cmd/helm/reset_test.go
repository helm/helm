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

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/proto/hapi/release"
)

type resetCase struct {
	name            string
	err             bool
	resp            []*release.Release
	removeHelmHome  bool
	force           bool
	expectedActions int
	expectedOutput  string
}

func TestResetCmd(t *testing.T) {

	verifyResetCmd(t, resetCase{
		name:            "test reset command",
		expectedActions: 3,
		expectedOutput:  "Tiller (the Helm server-side component) has been uninstalled from your Kubernetes Cluster.",
	})
}

func TestResetCmd_removeHelmHome(t *testing.T) {
	verifyResetCmd(t, resetCase{
		name:            "test reset command - remove helm home",
		removeHelmHome:  true,
		expectedActions: 3,
		expectedOutput:  "Tiller (the Helm server-side component) has been uninstalled from your Kubernetes Cluster.",
	})
}

func TestReset_deployedReleases(t *testing.T) {
	verifyResetCmd(t, resetCase{
		name: "test reset command - deployed releases",
		resp: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
		},
		err:            true,
		expectedOutput: "there are still 1 deployed releases (Tip: use --force to remove Tiller. Releases will not be deleted.)",
	})
}

func TestReset_forceFlag(t *testing.T) {
	verifyResetCmd(t, resetCase{
		name:  "test reset command - force flag",
		force: true,
		resp: []*release.Release{
			helm.ReleaseMock(&helm.MockReleaseOptions{Name: "atlas-guide", StatusCode: release.Status_DEPLOYED}),
		},
		expectedActions: 3,
		expectedOutput:  "Tiller (the Helm server-side component) has been uninstalled from your Kubernetes Cluster.",
	})
}

func verifyResetCmd(t *testing.T, tc resetCase) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	c := &helm.FakeClient{
		Rels: tc.resp,
	}
	fc := fake.NewSimpleClientset()
	cmd := &resetCmd{
		removeHelmHome: tc.removeHelmHome,
		force:          tc.force,
		out:            &buf,
		home:           helmpath.Home(home),
		client:         c,
		kubeClient:     fc,
		namespace:      v1.NamespaceDefault,
	}

	err = cmd.run()
	if !tc.err && err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	got := buf.String()
	if tc.err {
		got = err.Error()
	}

	actions := fc.Actions()
	if tc.expectedActions > 0 && len(actions) != tc.expectedActions {
		t.Errorf("Expected %d actions, got %d", tc.expectedActions, len(actions))
	}
	if !strings.Contains(got, tc.expectedOutput) {
		t.Errorf("expected %q, got %q", tc.expectedOutput, got)
	}
	_, err = os.Stat(home)
	if !tc.removeHelmHome && err != nil {
		t.Errorf("Helm home directory %s does not exist", home)
	}
	if tc.removeHelmHome && err == nil {
		t.Errorf("Helm home directory %s exists", home)
	}
}
