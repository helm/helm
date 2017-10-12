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
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ghodss/yaml"

	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	testcore "k8s.io/client-go/testing"

	"encoding/json"

	"k8s.io/helm/cmd/helm/installer"
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
		namespace:  v1.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("expected error: %v", err)
	}
	actions := fc.Actions()
	if len(actions) != 2 {
		t.Errorf("Expected 2 actions, got %d", len(actions))
	}
	if !actions[0].Matches("create", "deployments") {
		t.Errorf("unexpected action: %v, expected create deployment", actions[0])
	}
	if !actions[1].Matches("create", "services") {
		t.Errorf("unexpected action: %v, expected create service", actions[1])
	}
	expected := "Tiller (the Helm server-side component) has been installed into your Kubernetes Cluster."
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
	fc := fake.NewSimpleClientset(&v1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: v1.NamespaceDefault,
			Name:      "tiller-deploy",
		},
	})
	fc.PrependReactor("*", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(v1.Resource("deployments"), "1")
	})
	cmd := &initCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		kubeClient: fc,
		namespace:  v1.NamespaceDefault,
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
		namespace:  v1.NamespaceDefault,
	}
	if err := cmd.run(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(fc.Actions()) != 0 {
		t.Error("expected client call")
	}
	expected := "Not installing Tiller due to 'client-only' flag having been set"
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
	cleanup := resetEnv()
	defer func() {
		os.Remove(home)
		cleanup()
	}()

	settings.Debug = true

	var buf bytes.Buffer
	fc := fake.NewSimpleClientset()
	cmd := &initCmd{
		out:        &buf,
		home:       helmpath.Home(home),
		kubeClient: fc,
		clientOnly: true,
		dryRun:     true,
		namespace:  v1.NamespaceDefault,
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
	settings.Home = hh
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

	if fi, err := os.Stat(hh.LocalRepository(localRepositoryIndexFile)); err != nil {
		t.Errorf("%s", err)
	} else if fi.IsDir() {
		t.Errorf("%s should not be a directory", fi)
	}
}

func TestInitCmd_tlsOptions(t *testing.T) {
	const testDir = "../../testdata"

	// tls certificates in testDir
	var (
		testCaCertFile = filepath.Join(testDir, "ca.pem")
		testCertFile   = filepath.Join(testDir, "crt.pem")
		testKeyFile    = filepath.Join(testDir, "key.pem")
	)

	// these tests verify the effects of permuting the "--tls" and "--tls-verify" flags
	// and the install options yieled as a result of (*initCmd).tlsOptions()
	// during helm init.
	var tests = []struct {
		certFile string
		keyFile  string
		caFile   string
		enable   bool
		verify   bool
		describe string
	}{
		{ // --tls and --tls-verify specified (--tls=true,--tls-verify=true)
			certFile: testCertFile,
			keyFile:  testKeyFile,
			caFile:   testCaCertFile,
			enable:   true,
			verify:   true,
			describe: "--tls and --tls-verify specified (--tls=true,--tls-verify=true)",
		},
		{ // --tls-verify implies --tls (--tls=false,--tls-verify=true)
			certFile: testCertFile,
			keyFile:  testKeyFile,
			caFile:   testCaCertFile,
			enable:   false,
			verify:   true,
			describe: "--tls-verify implies --tls (--tls=false,--tls-verify=true)",
		},
		{ // no --tls-verify (--tls=true,--tls-verify=false)
			certFile: testCertFile,
			keyFile:  testKeyFile,
			caFile:   "",
			enable:   true,
			verify:   false,
			describe: "no --tls-verify (--tls=true,--tls-verify=false)",
		},
		{ // tls is disabled (--tls=false,--tls-verify=false)
			certFile: "",
			keyFile:  "",
			caFile:   "",
			enable:   false,
			verify:   false,
			describe: "tls is disabled (--tls=false,--tls-verify=false)",
		},
	}

	for _, tt := range tests {
		// emulate tls file specific flags
		tlsCaCertFile, tlsCertFile, tlsKeyFile = tt.caFile, tt.certFile, tt.keyFile

		// emulate tls enable/verify flags
		tlsEnable, tlsVerify = tt.enable, tt.verify

		cmd := &initCmd{}
		if err := cmd.tlsOptions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// expected result options
		expect := installer.Options{
			TLSCaCertFile: tt.caFile,
			TLSCertFile:   tt.certFile,
			TLSKeyFile:    tt.keyFile,
			VerifyTLS:     tt.verify,
			EnableTLS:     tt.enable || tt.verify,
		}

		if !reflect.DeepEqual(cmd.opts, expect) {
			t.Errorf("%s: got %#+v, want %#+v", tt.describe, cmd.opts, expect)
		}
	}
}

// TestInitCmd_output tests that init -o formats are unmarshal-able
func TestInitCmd_output(t *testing.T) {
	// This is purely defensive in this case.
	home, err := ioutil.TempDir("", "helm_home")
	if err != nil {
		t.Fatal(err)
	}
	dbg := settings.Debug
	settings.Debug = true
	defer func() {
		os.Remove(home)
		settings.Debug = dbg
	}()
	fc := fake.NewSimpleClientset()
	tests := []struct {
		expectF    func([]byte, interface{}) error
		expectName string
	}{
		{
			json.Unmarshal,
			"json",
		},
		{
			yaml.Unmarshal,
			"yaml",
		},
	}
	for _, s := range tests {
		var buf bytes.Buffer
		cmd := &initCmd{
			out:        &buf,
			home:       helmpath.Home(home),
			kubeClient: fc,
			opts:       installer.Options{Output: installer.OutputFormat(s.expectName)},
			namespace:  v1.NamespaceDefault,
		}
		if err := cmd.run(); err != nil {
			t.Fatal(err)
		}
		if got := len(fc.Actions()); got != 0 {
			t.Errorf("expected no server calls, got %d", got)
		}
		d := &v1beta1.Deployment{}
		if err = s.expectF(buf.Bytes(), &d); err != nil {
			t.Errorf("error unmarshalling init %s output %s %s", s.expectName, err, buf.String())
		}
	}

}
