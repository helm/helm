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

	"github.com/ghodss/yaml"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	testcore "k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"

	"k8s.io/helm/pkg/helm/helmpath"
)

func TestInitCmd(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	fc := fake.NewSimpleClientset()
	cmd := &initCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		kubeClient: fc,
		namespace:  api.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("expected error: %v", err)
	}
	actions := fc.Actions()
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(actions))
	}
	if !actions[0].Matches("create", "services") {
		t.Errorf("unexpected action: %v, expected create service", actions[0])
	}
	if !actions[1].Matches("create", "deployments") {
		t.Errorf("unexpected action: %v, expected create deployment", actions[1])
	}
	expected := "Tiller (the helm server side component) has been installed into your Kubernetes Cluster."
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestInitCmd_exists(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	fc := fake.NewSimpleClientset(&extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Namespace: api.NamespaceDefault,
			Name:      "tiller-deploy",
		},
	})
	fc.PrependReactor("*", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewAlreadyExists(api.Resource("deployments"), "1")
	})
	cmd := &initCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		kubeClient: fc,
		namespace:  api.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("expected error: %v", err)
	}
	expected := "Warning: Tiller is already installed in the cluster.\n" +
		"(Use --client-only to suppress this message, or --upgrade to upgrade Tiller to the current version.)"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestInitCmd_clientOnly(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	fc := fake.NewSimpleClientset()
	cmd := &initCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		kubeClient: fc,
		clientOnly: true,
		namespace:  api.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(fc.Actions()) != 0 {
		t.Error("expected client call")
	}
	expected := "Not installing tiller due to 'client-only' flag having been set"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestInitCmd_dryRun(t *testing.T) {
	// This is purely defensive in this case.
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	dbg := flagDebug
	flagDebug = true
	defer func() {
		os.Remove(home)
		flagDebug = dbg
	}()

	var buf bytes.Buffer
	fc := fake.NewSimpleClientset()
	cmd := &initCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		kubeClient: fc,
		clientOnly: true,
		dryRun:     true,
		namespace:  api.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Fatal(err)
	}
	if got := len(fc.Actions()); got != 0 {
		t.Errorf("expected no server calls, got %d", got)
	}

	docs := bytes.Split(buf.Bytes(), []byte("\n---"))
	if got, want := len(docs), 2; got != want {
		t.Fatalf("Expected document count of %d, got %d", want, got)
	}
	for _, doc := range docs {
		var y map[string]interface{}
		if err := yaml.Unmarshal(doc, &y); err != nil {
			t.Errorf("Expected parseable YAML, got %q\n\t%s", doc, err)
		}
	}
}

func TestEnsureHome(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	b := bytes.NewBuffer(nil)
	hh := helmpath.Home(home)
	helmHome = home
	if err := ensureDirectories(hh, b); err != nil {
		t.Error(err)
	}
	if err := ensureDefaultRepos(hh, b, false); err != nil {
		t.Error(err)
	}
	if err := ensureDefaultRepos(hh, b, true); err != nil {
		t.Error(err)
	}
	if err := ensureRepoFileFormat(hh.RepositoryFile(), b); err != nil {
		t.Error(err)
	}

	expectedDirs := []string{hh.String(), hh.Repository(), hh.Cache(), hh.LocalRepository()}
	for _, dir := range expectedDirs {
		if fi, err := os.Stat(dir); err != nil {
			t.Errorf("%s", err)
		} else if !fi.IsDir() {
			t.Errorf("%s is not a directory", fi)
		}
	}

	if fi, err := os.Stat(hh.RepositoryFile()); err != nil {
		t.Error(err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}

	if fi, err := os.Stat(hh.LocalRepository(localRepoIndexFilePath)); err != nil {
		t.Errorf("%s", err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}
}
