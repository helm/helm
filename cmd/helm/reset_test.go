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

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/helmpath"
	"k8s.io/helm/pkg/proto/hapi/release"
)

func TestResetCmd(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	c := &helm.FakeClient{}
	fc := fake.NewSimpleClientset()
	cmd := &resetCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		client:     c,
		kubeClient: fc,
		namespace:  api.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actions := fc.Actions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
	}
	expected := "Tiller (the Helm server-side component) has been uninstalled from your Kubernetes Cluster."
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
	if _, err := os.Stat(home); err != nil {
		t.Errorf("Helm home directory %s does not exists", home)
	}
}

func TestResetCmd_removeHelmHome(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	c := &helm.FakeClient{}
	fc := fake.NewSimpleClientset()
	cmd := &resetCmd{
		removeHelmHome: true,
		out:            &buf,
		home:           helmpath.Home(home),
		client:         c,
		kubeClient:     fc,
		namespace:      api.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actions := fc.Actions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
	}
	expected := "Tiller (the Helm server-side component) has been uninstalled from your Kubernetes Cluster."
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
	if _, err := os.Stat(home); err == nil {
		t.Errorf("Helm home directory %s already exists", home)
	}
}

func TestReset_deployedReleases(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	resp := []*release.Release{
		releaseMock(&releaseOptions{name: "atlas-guide", statusCode: release.Status_DEPLOYED}),
	}
	c := &helm.FakeClient{
		Rels: resp,
	}
	fc := fake.NewSimpleClientset()
	cmd := &resetCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		client:     c,
		kubeClient: fc,
		namespace:  api.NamespaceDefault,
	}
	err = cmd.run()
	expected := "there are still 1 deployed releases (Tip: use --force)"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(home); err != nil {
		t.Errorf("Helm home directory %s does not exists", home)
	}
}

func TestReset_forceFlag(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	resp := []*release.Release{
		releaseMock(&releaseOptions{name: "atlas-guide", statusCode: release.Status_DEPLOYED}),
	}
	c := &helm.FakeClient{
		Rels: resp,
	}
	fc := fake.NewSimpleClientset()
	cmd := &resetCmd{
		force:      true,
		out:        &buf,
		home:       helmpath.Home(home),
		client:     c,
		kubeClient: fc,
		namespace:  api.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	actions := fc.Actions()
	if len(actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(actions))
	}
	expected := "Tiller (the Helm server-side component) has been uninstalled from your Kubernetes Cluster."
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
	if _, err := os.Stat(home); err != nil {
		t.Errorf("Helm home directory %s does not exists", home)
	}
}
