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
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"k8s.io/helm/cmd/helm/helmpath"
)

func TestInitCmd(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	fake := testclient.Fake{}
	cmd := &initCmd{out: &buf, home: helmpath.Home(home), kubeClient: fake.Extensions()}
	if err := cmd.run(); err != nil {
		t.Errorf("expected error: %v", err)
	}
	actions := fake.Actions()
	if action, ok := actions[0].(testclient.CreateAction); !ok || action.GetResource() != "deployments" {
		t.Errorf("unexpected action: %v, expected create deployment", actions[0])
	}
	expected := "Tiller (the helm server side component) has been installed into your Kubernetes Cluster."
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestInitCmd_exsits(t *testing.T) {
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(home)

	var buf bytes.Buffer
	fake := testclient.Fake{}
	fake.AddReactor("*", "*", func(action testclient.Action) (bool, runtime.Object, error) {
		return true, nil, errors.NewAlreadyExists(api.Resource("deployments"), "1")
	})
	cmd := &initCmd{out: &buf, home: helmpath.Home(home), kubeClient: fake.Extensions()}
	if err := cmd.run(); err != nil {
		t.Errorf("expected error: %v", err)
	}
	expected := "Warning: Tiller is already installed in the cluster. (Use --client-only to suppress this message.)"
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
	fake := testclient.Fake{}
	cmd := &initCmd{out: &buf, home: helmpath.Home(home), kubeClient: fake.Extensions(), clientOnly: true}
	if err := cmd.run(); err != nil {
		t.Errorf("expected error: %v", err)
	}
	if len(fake.Actions()) != 0 {
		t.Error("expected client call")
	}
	expected := "Not installing tiller due to 'client-only' flag having been set"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected %q, got %q", expected, buf.String())
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
	if err := ensureHome(hh, b); err != nil {
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
